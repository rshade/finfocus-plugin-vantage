package adapter

import (
	"context"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantage/client"
)

// mapVantageRowToCostRecord converts a Vantage CostRow to a FinFocus CostRecord.
func (a *Adapter) mapVantageRowToCostRecord(
	row client.CostRow,
	query client.Query,
	queryHash, metricType string,
) CostRecord {
	// Generate idempotency key for line_item_id (FOCUS 1.2 requirement).
	lineItemID := GenerateLineItemID(query.CostReportToken, row, query.Metrics)

	record := CostRecord{
		Timestamp:         row.BucketStart,
		Provider:          row.Provider,
		Service:           row.Service,
		AccountID:         row.Account,
		Project:           row.Project,
		Region:            row.Region,
		ResourceID:        row.ResourceID,
		Currency:          row.Currency,
		SourceReportToken: query.CostReportToken,
		QueryHash:         queryHash,
		LineItemID:        lineItemID,
		MetricType:        metricType,
		Diagnostics:       &Diagnostics{},
	}

	// Map usage metrics.
	if row.UsageQuantity != 0 {
		record.UsageAmount = &row.UsageQuantity
	}
	record.UsageUnit = row.UsageUnit

	// Map cost metrics.
	if row.ListCost != 0 {
		record.ListCost = &row.ListCost
	}
	if row.Cost != 0 {
		record.NetCost = &row.Cost
	}
	if row.AmortizedCost != 0 {
		record.AmortizedCost = &row.AmortizedCost
	}
	if row.Tax != 0 {
		record.TaxCost = &row.Tax
	}
	if row.Credit != 0 {
		record.CreditAmount = &row.Credit
	}
	if row.Refund != 0 {
		record.RefundAmount = &row.Refund
	}

	// Normalize and map tags.
	record.Labels = a.normalizeTags(row.Tags)

	// Add diagnostics for missing fields.
	a.addDiagnostics(&record, row)

	return record
}

// addDiagnostics adds diagnostic information for missing or problematic fields.
func (a *Adapter) addDiagnostics(record *CostRecord, _ client.CostRow) {
	diag := record.Diagnostics

	// Check for missing required FOCUS 1.2 fields.
	if record.Provider == "" {
		reason := "required FOCUS 1.2 field cloud_provider is empty"
		diag.AddMissingField("provider", reason)
		a.logMissingField("provider", reason, record)
	}
	if record.Service == "" {
		reason := "required FOCUS 1.2 field service_name is empty"
		diag.AddMissingField("service", reason)
		a.logMissingField("service", reason, record)
	}
	if record.AccountID == "" {
		reason := "FOCUS 1.2 field billing_account_id is empty"
		diag.AddMissingField("account_id", reason)
		a.logMissingField("account_id", reason, record)
	}
	if record.Region == "" {
		reason := "FOCUS 1.2 field region is empty"
		diag.AddMissingField("region", reason)
		a.logMissingField("region", reason, record)
	}
	if record.Currency == "" {
		reason := "FOCUS 1.2 field billing_currency is empty"
		diag.AddMissingField("currency", reason)
		a.logMissingField("currency", reason, record)
	}
	if record.NetCost == nil || *record.NetCost == 0 {
		reason := "required FOCUS 1.2 field net_cost is nil or zero"
		diag.AddMissingField("net_cost", reason)
		a.logMissingField("net_cost", reason, record)
	}

	// Check for usage metric inconsistencies.
	if record.UsageAmount != nil && *record.UsageAmount != 0 && record.UsageUnit == "" {
		warning := "usage_amount_present_but_unit_missing"
		diag.AddWarning(warning)
		a.logWarning(warning, "FOCUS 1.2 field usage_unit missing when usage_amount is present", record)
	}
	if record.UsageAmount == nil && record.UsageUnit != "" {
		warning := "usage_unit_present_but_amount_missing"
		diag.AddWarning(warning)
		a.logWarning(warning, "FOCUS 1.2 field usage_amount missing when usage_unit is present", record)
	}

	// Check for unusual cost values.
	if record.NetCost != nil && *record.NetCost < 0 {
		warning := "negative_net_cost"
		diag.AddWarning(warning)
		a.logWarning(warning, "net_cost is negative, may indicate refund or credit", record)
	}
	if record.ListCost != nil && record.NetCost != nil && *record.ListCost < *record.NetCost {
		warning := "list_cost_less_than_net_cost"
		diag.AddWarning(warning)
		a.logWarning(warning, "list_cost is less than net_cost, unusual pattern", record)
	}

	// Check for resource identification issues.
	if record.ResourceID == "" && record.Service != "" {
		warning := "missing_resource_id"
		diag.AddWarning(warning)
		a.logWarning(warning, "FOCUS 1.2 field resource_id is empty for service", record)
	}

	// If no diagnostics were added, set to nil.
	if !diag.HasIssues() {
		record.Diagnostics = nil
	}
}

// logMissingField logs a missing field diagnostic with structured fields.
func (a *Adapter) logMissingField(fieldName, reason string, record *CostRecord) {
	a.logger.Warn(context.TODO(), "Missing field detected", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpFieldValid,
		"field":           fieldName,
		"reason":          reason,
		"timestamp":       record.Timestamp.Format("2006-01-02"),
		logFieldProvider:  record.Provider,
		logFieldService:   record.Service,
	})
}

// logWarning logs a diagnostic warning with structured fields.
func (a *Adapter) logWarning(warning, description string, record *CostRecord) {
	a.logger.Warn(context.TODO(), "Data quality warning", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: "data_validation",
		"warning":         warning,
		"description":     description,
		"timestamp":       record.Timestamp.Format("2006-01-02"),
		logFieldProvider:  record.Provider,
		logFieldService:   record.Service,
	})
}
