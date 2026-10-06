// Package adapter provides the Vantage adapter for FinFocus.
package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantage/client"
)

// CostRecord represents a cost record in FinFocus's internal schema with FOCUS 1.2 fields.
type CostRecord struct {
	// Core dimensions.
	Timestamp      time.Time         `json:"timestamp"`
	Provider       string            `json:"provider,omitempty"`
	Service        string            `json:"service,omitempty"`
	AccountID      string            `json:"account_id,omitempty"`
	SubscriptionID string            `json:"subscription_id,omitempty"`
	Project        string            `json:"project,omitempty"`
	Region         string            `json:"region,omitempty"`
	ResourceID     string            `json:"resource_id,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`

	// Usage metrics.
	UsageAmount *float64 `json:"usage_amount,omitempty"`
	UsageUnit   string   `json:"usage_unit,omitempty"`

	// Cost metrics.
	ListCost      *float64 `json:"list_cost,omitempty"`
	NetCost       *float64 `json:"net_cost,omitempty"`
	AmortizedCost *float64 `json:"amortized_cost,omitempty"`
	TaxCost       *float64 `json:"tax_cost,omitempty"`
	CreditAmount  *float64 `json:"credit_amount,omitempty"`
	RefundAmount  *float64 `json:"refund_amount,omitempty"`

	// Metadata.
	Currency          string `json:"currency,omitempty"`
	SourceReportToken string `json:"source_report_token,omitempty"`
	QueryHash         string `json:"query_hash"`
	LineItemID        string `json:"line_item_id"`          // FOCUS 1.2 idempotency key (report_token, date, dimensions, metrics hash)
	MetricType        string `json:"metric_type,omitempty"` // "cost" or "forecast"

	// Diagnostics.
	Diagnostics *Diagnostics `json:"diagnostics,omitempty"`
}

// Log field constants for structured logging.
const (
	logFieldAdapter      = "adapter"
	logFieldOperation    = "operation"
	logFieldAttempt      = "attempt"
	logFieldError        = "error"
	logFieldRows         = "rows"
	logFieldProvider     = "provider"
	logFieldService      = "service"
	logFieldTotalRecords = "total_records"
	logFieldSyncSummary  = "sync_summary"
	logFieldCostsRequest = "costs_request"
	logFieldForecastReq  = "forecast_request"

	// Log values.
	logAdapterVantage = "vantage"
	logOpSync         = "sync"
	logOpSyncIncr     = "incremental_sync"
	logOpFieldValid   = "field_validation"
	logOpCostsRequest = "costs_request"
	logOpForecastReq  = "forecast_request"
	logOpSyncSummary  = "sync_summary"
)

// Config dimension fields.
const (
	dimProvider = "provider"
	dimService  = "service"
)

// Sink defines the interface for persisting cost records.
// This interface is assumed to exist in finfocus.
type Sink interface {
	// WriteRecords writes cost records to the data store.
	WriteRecords(ctx context.Context, records []CostRecord) error

	// GetBookmark retrieves the last successful sync bookmark.
	GetBookmark(ctx context.Context, key string) (string, error)

	// SetBookmark stores the last successful sync bookmark.
	SetBookmark(ctx context.Context, key string, value string) error
}

// Adapter implements the Vantage adapter for FinFocus.
type Adapter struct {
	client             client.Client
	logger             client.Logger
	diagnosticsSummary *DiagnosticsSummary
}

// New creates a new Vantage adapter.
func New(client client.Client, logger client.Logger) *Adapter {
	return &Adapter{
		client:             client,
		logger:             logger,
		diagnosticsSummary: NewDiagnosticsSummary(),
	}
}

// GetDiagnosticsSummary returns the aggregated diagnostics from the last sync operation.
func (a *Adapter) GetDiagnosticsSummary() *DiagnosticsSummary {
	return a.diagnosticsSummary
}

// ResetDiagnosticsSummary resets the diagnostics summary for a new sync operation.
func (a *Adapter) ResetDiagnosticsSummary() {
	a.diagnosticsSummary = NewDiagnosticsSummary()
}

// Sync performs a cost data sync operation.
func (a *Adapter) Sync(ctx context.Context, cfg Config, sink Sink) error {
	// Reset diagnostics summary for this sync operation.
	a.ResetDiagnosticsSummary()

	a.logger.Info(ctx, "Starting Vantage adapter sync", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpSync,
		logFieldAttempt:   0,
	})

	// Determine sync mode based on configuration.
	var err error
	if cfg.EndDate == nil {
		// Incremental sync: D-3 to D-1.
		err = a.syncIncremental(ctx, cfg, sink)
	} else {
		// Backfill sync: specified date range.
		err = a.syncBackfill(ctx, cfg, sink)
	}

	// Log diagnostic summary after sync completes, passing the error.
	a.logDiagnosticsSummary(ctx, err)

	return err
}

// syncIncremental performs incremental sync with D-3 to D-1 lag window.
func (a *Adapter) syncIncremental(ctx context.Context, cfg Config, sink Sink) error {
	now := time.Now().UTC()
	startDate := now.AddDate(0, 0, -3) // D-3
	endDate := now.AddDate(0, 0, -1)   // D-1

	a.logger.Info(ctx, "Performing incremental sync", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpSyncIncr,
		logFieldAttempt:   0,
		"start_date":      startDate.Format("2006-01-02"),
		"end_date":        endDate.Format("2006-01-02"),
	})

	return a.syncDateRange(ctx, cfg, sink, startDate, endDate, false)
}

// syncBackfill performs backfill sync for the specified date range.
func (a *Adapter) syncBackfill(ctx context.Context, cfg Config, sink Sink) error {
	startDate := cfg.StartDate
	endDate := *cfg.EndDate

	a.logger.Info(ctx, "Performing backfill sync", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: "backfill_sync",
		logFieldAttempt:   0,
		"start_date":      startDate.Format("2006-01-02"),
		"end_date":        endDate.Format("2006-01-02"),
	})

	return a.syncDateRange(ctx, cfg, sink, startDate, endDate, true)
}

// syncDateRange syncs data for a specific date range.
func (a *Adapter) syncDateRange(
	ctx context.Context,
	cfg Config,
	sink Sink,
	startDate, endDate time.Time,
	isBackfill bool,
) error {
	// For backfill, chunk by month to limit payload size.
	if isBackfill && endDate.Sub(startDate).Hours() > 24*30 {
		return a.syncChunked(ctx, cfg, sink, startDate, endDate)
	}

	// Single range sync.
	return a.syncSingleRange(ctx, cfg, sink, startDate, endDate, isBackfill)
}

// syncChunked performs chunked sync by month for large date ranges.
func (a *Adapter) syncChunked(ctx context.Context, cfg Config, sink Sink, startDate, endDate time.Time) error {
	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)

	for current.Before(endDate) {
		chunkEnd := time.Date(current.Year(), current.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		if chunkEnd.After(endDate) {
			chunkEnd = endDate
		}

		if err := a.syncSingleRange(ctx, cfg, sink, current, chunkEnd, true); err != nil {
			return fmt.Errorf(
				"syncing chunk %s to %s: %w",
				current.Format("2006-01-02"),
				chunkEnd.Format("2006-01-02"),
				err,
			)
		}

		current = chunkEnd
	}

	return nil
}

// syncSingleRange syncs a single date range.
func (a *Adapter) syncSingleRange(
	ctx context.Context,
	cfg Config,
	sink Sink,
	startDate, endDate time.Time,
	isBackfill bool,
) error {
	query := client.Query{
		WorkspaceToken:  cfg.WorkspaceToken,
		CostReportToken: cfg.CostReportToken,
		StartAt:         startDate,
		EndAt:           endDate,
		Granularity:     cfg.Granularity,
		GroupBys:        cfg.GroupBys,
		Metrics:         cfg.Metrics,
		PageSize:        cfg.PageSize,
	}

	// Generate idempotency key.
	queryHash := a.generateQueryHash(query)
	bookmarkKey := fmt.Sprintf("vantage_%s", queryHash)

	// Apply bookmark for incremental sync.
	a.applyBookmark(ctx, &query, sink, bookmarkKey, isBackfill)

	// Fetch and collect all records.
	allRecords, pageCount, err := a.fetchAndCollectRecords(ctx, query, queryHash)
	if err != nil {
		return err
	}

	a.logger.Info(ctx, "Fetched cost data", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: "fetch_cost_data",
		logFieldAttempt:   0,
		"pages":           pageCount,
		"records":         len(allRecords),
		"query_hash":      queryHash,
	})

	// Write records.
	if err = sink.WriteRecords(ctx, allRecords); err != nil {
		return fmt.Errorf("writing records: %w", err)
	}

	// Update bookmark for incremental sync.
	a.updateBookmark(ctx, sink, bookmarkKey, endDate, isBackfill)

	// Handle forecast if enabled.
	a.handleForecast(ctx, cfg, sink, startDate, endDate, queryHash)

	return nil
}

// applyBookmark applies the last saved bookmark to resume from a previous sync.
func (a *Adapter) applyBookmark(
	ctx context.Context,
	query *client.Query,
	sink Sink,
	bookmarkKey string,
	isBackfill bool,
) {
	if isBackfill {
		return
	}

	lastEndDate, err := sink.GetBookmark(ctx, bookmarkKey)
	if err == nil && lastEndDate != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, lastEndDate); parseErr == nil {
			query.StartAt = parsed
			a.logger.Info(ctx, "Resuming from bookmark", map[string]interface{}{
				logFieldAdapter:   logAdapterVantage,
				logFieldOperation: "resume_bookmark",
				logFieldAttempt:   0,
				"bookmark":        lastEndDate,
			})
		}
	}
}

// fetchAndCollectRecords fetches pages of data and collects them into records.
func (a *Adapter) fetchAndCollectRecords(
	ctx context.Context,
	query client.Query,
	queryHash string,
) ([]CostRecord, int, error) {
	pager := client.NewPager(a.client, query, a.logger)

	var allRecords []CostRecord
	pageCount := 0

	for pager.HasMore() || pageCount == 0 {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("fetching page: %w", err)
		}

		// Convert Vantage rows to CostRecords.
		for _, row := range page.Data {
			record := a.mapVantageRowToCostRecord(row, query, queryHash, "cost")
			allRecords = append(allRecords, record)
			a.diagnosticsSummary.AddRecordDiagnostics(record.Diagnostics)
		}

		pageCount++
		if !page.HasMore {
			break
		}
	}

	return allRecords, pageCount, nil
}

// updateBookmark saves the last end date for incremental syncs.
func (a *Adapter) updateBookmark(
	ctx context.Context,
	sink Sink,
	bookmarkKey string,
	endDate time.Time,
	isBackfill bool,
) {
	if isBackfill {
		return
	}

	bookmarkValue := endDate.Format(time.RFC3339)
	if err := sink.SetBookmark(ctx, bookmarkKey, bookmarkValue); err != nil {
		a.logger.Warn(ctx, "Failed to update bookmark", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "update_bookmark",
			logFieldAttempt:   0,
			logFieldError:     err,
		})
	}
}

// handleForecast syncs forecast data if enabled.
func (a *Adapter) handleForecast(
	ctx context.Context,
	cfg Config,
	sink Sink,
	startDate, endDate time.Time,
	queryHash string,
) {
	if !cfg.IncludeForecast || cfg.CostReportToken == "" {
		return
	}

	if err := a.syncForecast(ctx, cfg, sink, startDate, endDate, queryHash); err != nil {
		a.logger.Warn(ctx, "Forecast sync failed", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "forecast_sync",
			logFieldAttempt:   0,
			logFieldError:     err,
		})
	}
}

// syncForecast syncs forecast data for the given date range.
func (a *Adapter) syncForecast(
	ctx context.Context,
	cfg Config,
	sink Sink,
	startDate, endDate time.Time,
	queryHash string,
) error {
	forecastQuery := client.ForecastQuery{
		StartAt:     startDate,
		EndAt:       endDate,
		Granularity: cfg.Granularity,
	}

	forecast, err := a.client.Forecast(ctx, cfg.CostReportToken, forecastQuery)
	if err != nil {
		return fmt.Errorf("fetching forecast: %w", err)
	}

	var forecastRecords []CostRecord
	for _, row := range forecast.Data {
		record := a.mapVantageRowToCostRecord(client.CostRow{
			BucketStart: row.BucketStart,
			BucketEnd:   row.BucketEnd,
			Cost:        row.Cost,
			Currency:    row.Currency,
		}, client.Query{
			CostReportToken: cfg.CostReportToken,
			Granularity:     cfg.Granularity,
		}, queryHash, "forecast")
		forecastRecords = append(forecastRecords, record)

		// Collect diagnostics for summary.
		a.diagnosticsSummary.AddRecordDiagnostics(record.Diagnostics)
	}

	a.logger.Info(ctx, "Fetched forecast data", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: "fetch_forecast_data",
		logFieldAttempt:   0,
		"records":         len(forecastRecords),
		"query_hash":      queryHash,
	})

	return sink.WriteRecords(ctx, forecastRecords)
}

// generateQueryHash creates a stable hash for idempotency.
func (a *Adapter) generateQueryHash(query client.Query) string {
	// Create a stable string representation.
	parts := []string{
		query.WorkspaceToken,
		query.CostReportToken,
		query.StartAt.Format(time.RFC3339),
		query.EndAt.Format(time.RFC3339),
		query.Granularity,
	}

	// Sort groupbys and metrics for consistency.
	groupBys := make([]string, len(query.GroupBys))
	copy(groupBys, query.GroupBys)
	sort.Strings(groupBys)
	parts = append(parts, strings.Join(groupBys, ","))

	metrics := make([]string, len(query.Metrics))
	copy(metrics, query.Metrics)
	sort.Strings(metrics)
	parts = append(parts, strings.Join(metrics, ","))

	// Generate hash.
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:16]) // First 32 hex chars
}

// logDiagnosticsSummary logs the aggregated diagnostics summary after sync completion.
// If err is non-nil, it logs an error/failure summary instead of a success message.
func (a *Adapter) logDiagnosticsSummary(ctx context.Context, err error) {
	summary := a.GetDiagnosticsSummary()

	// If sync failed, log error summary instead of success.
	if err != nil {
		a.logSyncFailure(ctx, summary, err)
		return
	}

	// Log summary overview for successful sync.
	a.logSyncSuccess(ctx, summary)
}

// logSyncFailure logs the error summary when sync fails.
func (a *Adapter) logSyncFailure(ctx context.Context, summary *DiagnosticsSummary, err error) {
	a.logger.Error(ctx, "Sync failed", map[string]interface{}{
		logFieldAdapter:      logAdapterVantage,
		logFieldOperation:    logOpSyncSummary,
		logFieldError:        err.Error(),
		logFieldTotalRecords: summary.TotalRecords,
		"records_with_issue": summary.RecordsWithIssues,
	})

	// Still log diagnostic details if there were data quality issues.
	if !summary.HasIssues() {
		return
	}

	a.logDiagnosticDetails(ctx, summary)
}

// logSyncSuccess logs the success summary when sync completes successfully.
func (a *Adapter) logSyncSuccess(ctx context.Context, summary *DiagnosticsSummary) {
	if summary.HasIssues() {
		a.logger.Warn(ctx, "Sync completed with data quality issues", map[string]interface{}{
			logFieldAdapter:      logAdapterVantage,
			logFieldOperation:    logOpSyncSummary,
			logFieldTotalRecords: summary.TotalRecords,
			"records_with_issue": summary.RecordsWithIssues,
			"missing_fields":     len(summary.MissingFields),
			"warnings":           len(summary.Warnings),
		})
		a.logDiagnosticDetails(ctx, summary)
		return
	}

	a.logger.Info(ctx, "Sync completed successfully with no data quality issues", map[string]interface{}{
		logFieldAdapter:      logAdapterVantage,
		logFieldOperation:    logOpSyncSummary,
		logFieldTotalRecords: summary.TotalRecords,
	})
}

// logDiagnosticDetails logs detailed diagnostic information.
func (a *Adapter) logDiagnosticDetails(ctx context.Context, summary *DiagnosticsSummary) {
	// Log detailed missing fields breakdown.
	if len(summary.MissingFields) > 0 {
		a.logger.Warn(ctx, "Missing fields summary", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "diagnostic_summary",
			"missing_fields":  summary.MissingFields,
		})
	}

	// Log detailed warnings breakdown.
	if len(summary.Warnings) > 0 {
		a.logger.Warn(ctx, "Warnings summary", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "diagnostic_summary",
			"warnings":        summary.Warnings,
		})
	}
}
