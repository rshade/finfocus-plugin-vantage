// Package plugin implements the FinFocus cost source service for Vantage.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	openapiruntime "github.com/go-openapi/runtime"
	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantageapi"
)

// Plugin is the Vantage cost source. Cost retrieval is implemented separately
// from the server skeleton so the API client can be injected and tested.
type Plugin struct {
	version         string
	clientFactory   func(string) (vantageapi.Client, error)
	costReportToken string
	logger          zerolog.Logger
}

// New creates a Vantage plugin instance.
func New(version string) *Plugin {
	return &Plugin{
		version:         version,
		costReportToken: strings.TrimSpace(os.Getenv("FINFOCUS_VANTAGE_COST_REPORT_TOKEN")),
		logger:          zerolog.New(os.Stderr).With().Timestamp().Logger(),
		clientFactory: func(token string) (vantageapi.Client, error) {
			return vantageapi.NewClient("", token)
		},
	}
}

// Name returns the plugin discovery name.
func (p *Plugin) Name() string { return "vantage" }

// GetProjectedCost is unsupported because Vantage provides actual billing data.
func (p *Plugin) GetProjectedCost(
	ctx context.Context,
	_ *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	p.logRPC(ctx, "get_projected_cost")
	return nil, status.Error(codes.Unimplemented, "projected cost is not supported")
}

// GetActualCost retrieves Vantage costs for the requested resource and period.
func (p *Plugin) GetActualCost(ctx context.Context, req *pbc.GetActualCostRequest) (*pbc.GetActualCostResponse, error) {
	p.logRPC(ctx, "get_actual_cost")
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	provider := requestProvider(req)
	if provider == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required in tags[provider] or a supported AWS ARN")
	}
	client, err := p.clientForRequest(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := p.queryCosts(ctx, client, req, provider)
	if err != nil {
		return nil, upstreamStatus(err)
	}
	return mapCosts(req, provider, resp)
}

func validateRequest(req *pbc.GetActualCostRequest) error {
	if req == nil || req.GetStart() == nil || req.GetEnd() == nil || req.GetResourceId() == "" {
		return status.Error(codes.InvalidArgument, "resource_id, start, and end are required")
	}
	if err := req.GetStart().CheckValid(); err != nil {
		return status.Error(codes.InvalidArgument, "invalid start timestamp")
	}
	if err := req.GetEnd().CheckValid(); err != nil {
		return status.Error(codes.InvalidArgument, "invalid end timestamp")
	}
	if !req.GetStart().AsTime().Before(req.GetEnd().AsTime()) {
		return status.Error(codes.InvalidArgument, "start must be before end")
	}
	for key := range req.GetTags() {
		if key != providerTag && key != serviceTag && !validLabelKey(key) {
			return status.Error(codes.InvalidArgument, "invalid Vantage label key")
		}
	}
	if _, err := requestPage(req.GetPageToken()); err != nil {
		return status.Error(codes.InvalidArgument, "invalid page token")
	}
	return nil
}

func requestProvider(req *pbc.GetActualCostRequest) string {
	provider := strings.TrimSpace(req.GetTags()["provider"])
	if provider == "" {
		provider = providerFromARN(req.GetArn())
	}
	return provider
}

func (p *Plugin) clientForRequest(ctx context.Context) (vantageapi.Client, error) {
	creds, err := pluginsdk.ExtractCredentials(ctx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid per-request credentials")
	}
	token, _ := creds.Get("vantage-token")
	if token == "" {
		token = strings.TrimSpace(os.Getenv("FINFOCUS_VANTAGE_TOKEN"))
	}
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "Vantage token is required")
	}
	if p.clientFactory == nil {
		return nil, status.Error(codes.FailedPrecondition, "Vantage client is not configured")
	}
	client, err := p.clientFactory(token)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Vantage client configuration is invalid")
	}
	if p.costReportToken == "" {
		return nil, status.Error(codes.FailedPrecondition, "Vantage cost report token is required")
	}
	return client, nil
}

func (p *Plugin) queryCosts(
	ctx context.Context,
	client vantageapi.Client,
	req *pbc.GetActualCostRequest,
	provider string,
) (*vantageapi.CostsResponse, error) {
	start, end := req.GetStart().AsTime(), req.GetEnd().AsTime()
	limit := req.GetPageSize()
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	page, err := requestPage(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page token")
	}
	filter := buildFilter(req, provider)
	params := &vantageapi.GetCostsParams{
		CostReportToken: &p.costReportToken,
		StartDate:       strPtr(start.UTC().Format("2006-01-02")), EndDate: strPtr(end.UTC().Format("2006-01-02")),
		DateBin: strPtr("day"), Groupings: []string{"provider", "service", "account_id", "region", "resource_id"},
		Filter: &filter, Limit: &limit, Page: &page,
	}
	for attempt := 0; ; attempt++ {
		resp, queryErr := client.GetCosts(ctx, params)
		if queryErr == nil || !isRateLimited(queryErr) || attempt >= maxRateLimitRetries {
			return resp, queryErr
		}
		delay := rateLimitBaseDelay << attempt
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

const (
	maxRateLimitRetries = 3
	rateLimitBaseDelay  = 200 * time.Millisecond
)

func isRateLimited(err error) bool {
	var apiErr *openapiruntime.APIError
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests
}

func upstreamStatus(err error) error {
	var apiErr *openapiruntime.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusUnauthorized, http.StatusForbidden:
			return status.Error(codes.Unauthenticated, "Vantage rejected the configured credential")
		case http.StatusBadRequest:
			return status.Error(codes.InvalidArgument, "Vantage rejected the cost query")
		}
	}
	return status.Errorf(codes.Unavailable, "Vantage cost query failed: %v", err)
}

func mapCosts(
	req *pbc.GetActualCostRequest,
	provider string,
	resp *vantageapi.CostsResponse,
) (*pbc.GetActualCostResponse, error) {
	result := &pbc.GetActualCostResponse{}
	currency := ""
	for _, row := range resp.Costs {
		if row == nil {
			continue
		}
		item, rowCurrency, err := mapCostRow(req, provider, row)
		if err != nil {
			return nil, err
		}
		if rowCurrency != "" {
			if currency != "" && currency != rowCurrency {
				return nil, status.Error(codes.FailedPrecondition, "Vantage returned mixed currencies")
			}
			currency = rowCurrency
		}
		result.Results = append(result.Results, item)
	}
	if resp.Links != nil && resp.Links.Next != nil {
		next, err := nextPageToken(*resp.Links.Next)
		if err != nil {
			return nil, status.Error(codes.DataLoss, "Vantage returned malformed pagination link")
		}
		result.NextPageToken = next
	}
	if result.GetNextPageToken() == "" {
		if len(result.GetResults()) > math.MaxInt32 {
			result.TotalCount = math.MaxInt32
		} else {
			result.TotalCount = int32(len(result.GetResults())) //nolint:gosec // bounded above by MaxInt32
		}
	}
	return result, nil
}

func mapCostRow(
	req *pbc.GetActualCostRequest,
	provider string,
	row *vantageapi.CostRow,
) (*pbc.ActualCostResult, string, error) {
	amount, err := strconv.ParseFloat(row.Amount, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, "", status.Error(codes.DataLoss, "Vantage returned malformed cost amount")
	}
	at, err := parseDate(row.AccruedAt)
	if err != nil {
		return nil, "", status.Error(codes.DataLoss, "Vantage returned malformed cost date")
	}
	item := &pbc.ActualCostResult{Timestamp: timestamppb.New(at), Cost: amount, Source: "vantage"}
	if focus := buildFocusRecord(req, provider, row, amount, at); focus != nil {
		item.FocusRecord = focus
	}
	return item, row.Currency, nil
}

func requestPage(token string) (int32, error) {
	if token == "" {
		return 1, nil
	}
	page, err := strconv.ParseInt(token, 10, 32)
	if err != nil || page < 1 {
		return 0, errors.New("invalid page token")
	}
	return int32(page), nil
}

func nextPageToken(nextURL string) (string, error) {
	u, err := url.Parse(nextURL)
	if err != nil {
		return "", err
	}
	page := u.Query().Get("page")
	if page == "" {
		return "", errors.New("next link does not contain page")
	}
	if _, pageErr := requestPage(page); pageErr != nil {
		return "", pageErr
	}
	return page, nil
}

func buildFocusRecord(
	req *pbc.GetActualCostRequest,
	provider string,
	row *vantageapi.CostRow,
	amount float64,
	at time.Time,
) *pbc.FocusCostRecord {
	account := ""
	if row.BillingAccountID != nil {
		account = *row.BillingAccountID
	}
	if account == "" && row.AccountID != nil {
		account = *row.AccountID
	}
	service := ""
	if row.Service != nil {
		service = *row.Service
	}
	if account == "" || service == "" || row.Currency == "" || row.Usage == nil || row.UsageUnit == nil ||
		strings.TrimSpace(*row.UsageUnit) == "" {
		return nil
	}
	usage, err := strconv.ParseFloat(fmt.Sprint(row.Usage), 64)
	if err != nil || usage <= 0 || math.IsNaN(usage) || math.IsInf(usage, 0) {
		return nil
	}
	start, end := req.GetStart().AsTime(), req.GetEnd().AsTime()
	builder := pluginsdk.NewFocusRecordBuilder().
		WithIdentity(provider, account, "").
		WithBillingPeriod(start, end, row.Currency).
		WithChargePeriod(at, at.Add(hoursPerDay*time.Hour)).
		WithChargeDetails(pbc.FocusChargeCategory_FOCUS_CHARGE_CATEGORY_USAGE, pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD).
		WithChargeClassification(pbc.FocusChargeClass_FOCUS_CHARGE_CLASS_REGULAR, service, pbc.FocusChargeFrequency_FOCUS_CHARGE_FREQUENCY_USAGE_BASED).
		WithFinancials(amount, amount, amount, row.Currency, "").
		WithService(pbc.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER, service).
		WithUsage(usage, *row.UsageUnit)
	if row.ResourceID != nil {
		builder.WithResource(*row.ResourceID, "", "")
	}
	record, err := builder.Build()
	if err != nil {
		return nil
	}
	return record
}

const (
	hoursPerDay = 24
	providerTag = "provider"
	serviceTag  = "service"
)

func strPtr(s string) *string { return &s }

func buildFilter(req *pbc.GetActualCostRequest, provider string) string {
	parts := []string{fmt.Sprintf("provider = '%s'", vqlQuote(provider))}
	if service := strings.TrimSpace(req.GetTags()[serviceTag]); service != "" {
		parts = append(parts, fmt.Sprintf("service = '%s'", vqlQuote(service)))
	}
	if req.GetResourceId() != "" {
		parts = append(parts, fmt.Sprintf("resource_id = '%s'", vqlQuote(req.GetResourceId())))
	}
	for k, v := range req.GetTags() {
		if k == providerTag || k == serviceTag {
			continue
		}
		parts = append(parts, fmt.Sprintf("labels.%s = '%s'", k, vqlQuote(v)))
	}
	return strings.Join(parts, " AND ")
}

func providerFromARN(arn string) string {
	parts := strings.SplitN(arn, ":", arnPartsCount)
	if len(parts) != arnPartsCount || parts[0] != "arn" {
		return ""
	}
	switch parts[2] {
	case "s3", "ec2", "lambda", "rds", "dynamodb":
		return "aws"
	default:
		return ""
	}
}

const arnPartsCount = 6

func validLabelKey(key string) bool {
	for i, r := range key {
		valid := r == '_' || r == '.' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			i > 0 && r >= '0' && r <= '9'
		if !valid {
			return false
		}
	}
	return key != ""
}

func vqlQuote(s string) string { return strings.ReplaceAll(s, "'", "\\'") }

func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", s)
}

// GetPricingSpec is unsupported because Vantage is not a pricing catalog.
func (p *Plugin) GetPricingSpec(
	ctx context.Context,
	_ *pbc.GetPricingSpecRequest,
) (*pbc.GetPricingSpecResponse, error) {
	p.logRPC(ctx, "get_pricing_spec")
	return nil, status.Error(codes.Unimplemented, "pricing specifications are not supported")
}

// EstimateCost is unsupported because this plugin reports imported actuals.
func (p *Plugin) EstimateCost(ctx context.Context, _ *pbc.EstimateCostRequest) (*pbc.EstimateCostResponse, error) {
	p.logRPC(ctx, "estimate_cost")
	return nil, status.Error(codes.Unimplemented, "cost estimation is not supported")
}

// Supports checks whether the resource provider can be represented in Vantage.
func (p *Plugin) Supports(ctx context.Context, req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	p.logRPC(ctx, "supports")
	if req == nil || req.GetResource() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource descriptor is required")
	}
	if req.GetResource().GetProvider() == "" || req.GetResource().GetResourceType() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider and resource type are required")
	}
	provider := strings.ToLower(req.GetResource().GetProvider())
	switch provider {
	case "aws", "azure", "gcp", "kubernetes", "custom":
		return &pbc.SupportsResponse{Supported: true}, nil
	default:
		return &pbc.SupportsResponse{
			Supported: false,
			Reason:    "provider is not a supported Vantage billing source",
		}, nil
	}
}

func (p *Plugin) logRPC(ctx context.Context, operation string) {
	traceID := pluginsdk.TraceIDFromContext(ctx)
	if traceID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(pluginsdk.TraceIDMetadataKey); len(values) > 0 {
				traceID = values[0]
			}
		}
	}
	p.logger.Info().Str(pluginsdk.FieldTraceID, traceID).Str("operation", operation).Msg("handling plugin request")
}

// ConsumesPerRequestCredentials opts into credentials in request metadata.
func (p *Plugin) ConsumesPerRequestCredentials() {}

var _ pluginsdk.PerRequestCredentialConsumer = (*Plugin)(nil)
