// Package vantageapi provides a thin wrapper around the official Vantage Go client.
package vantageapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/vantage-sh/vantage-go/vantagev2/models"
	"github.com/vantage-sh/vantage-go/vantagev2/vantage"
	"github.com/vantage-sh/vantage-go/vantagev2/vantage/costs"
)

// Client wraps the official Vantage generated client for cost queries.
// The plugin owns this interface to decouple from generated code changes.
type Client interface {
	GetCosts(ctx context.Context, params *GetCostsParams) (*CostsResponse, error)
}

// GetCostsParams represents parameters for the GET /costs operation.
type GetCostsParams struct {
	CostReportToken *string
	WorkspaceToken  *string
	Filter          *string
	StartDate       *string  // ISO 8601 format: YYYY-MM-DD
	EndDate         *string  // ISO 8601 format: YYYY-MM-DD
	DateBin         *string  // day, week, month, quarter, hour
	Groupings       []string // e.g., []string{"provider", "service", "account_id"}
	Page            *int32
	Limit           *int32
	// Settings for cost basis
	Settings *CostSettings
}

// CostSettings represents cost query settings.
type CostSettings struct {
	Amortize           *bool
	IncludeCredits     *bool
	IncludeRefunds     *bool
	IncludeDiscounts   *bool
	IncludeTax         *bool
	AggregateBy        *string
	ShowPreviousPeriod *bool
	UnAllocated        *bool
}

// CostsResponse represents the response from GET /costs.
type CostsResponse struct {
	// Costs is the list of cost rows; amounts are decimal strings
	Costs []*CostRow
	// TotalCost is the sum of all costs in the period
	TotalCost *CostAmount
	// TotalUsage is usage broken down by unit
	TotalUsage []*UsageAmount
	// Links for pagination (next URL, etc.)
	Links *PaginationLinks
}

// CostRow represents a single cost data point.
type CostRow struct {
	AccruedAt        string      // ISO 8601: YYYY-MM-DDTHH:MM:SSZ or YYYY-MM-DD
	Amount           string      // Decimal string, never float
	Currency         string      // ISO 4217 code (USD, EUR, etc.)
	Usage            interface{} // Untyped, provider-specific
	UsageUnit        *string
	Provider         *string
	Service          *string
	AccountID        *string
	BillingAccountID *string
	Region           *string
	ResourceID       *string
	ResourceName     *string
	Tags             []string // Array of strings, format undocumented
	CostCategory     *string
	CostSubcategory  *string
	ChargeType       *string // Distinguishes tax, credit, refund lines
	Tagged           *bool
	Segment          *string
}

// CostAmount represents an amount with currency.
type CostAmount struct {
	Amount   string // Decimal string
	Currency string // ISO 4217 code
}

// UsageAmount represents usage grouped by unit.
type UsageAmount struct {
	Amount string // Decimal string
	Unit   string
}

// PaginationLinks contains links for pagination.
type PaginationLinks struct {
	Self  *string
	First *string
	Next  *string
	Prev  *string
	Last  *string
}

// impl wraps the official vantage-go client and implements Client.
type impl struct {
	baseURL string
	token   string
	client  *vantage.Vantage
}

// NewClient creates a new Vantage API client with the given base URL and bearer token.
// If baseURL is empty, defaults to https://api.vantage.sh/v2.
func NewClient(baseURL, bearerToken string) (Client, error) {
	if bearerToken == "" {
		return nil, errors.New("bearer token is required")
	}

	if baseURL == "" {
		baseURL = "https://api.vantage.sh/v2"
	}

	// Detect scheme from URL and validate http usage
	schemes := []string{"https"}
	if strings.HasPrefix(baseURL, "http://") {
		// http:// is only allowed for loopback addresses (testing/development)
		host, _ := parseBaseURL(baseURL)
		if !isLoopbackHost(host) {
			return nil, errors.New("http:// scheme only allowed for loopback addresses (localhost, 127.0.0.1, ::1)")
		}
		schemes = []string{"http"}
	}

	// Parse base URL into host and path
	host, basePath := parseBaseURL(baseURL)

	// Create transport with custom auth
	transport := httptransport.New(host, basePath, schemes)

	// Set up bearer token authentication
	transport.DefaultAuthentication = httptransport.BearerToken(bearerToken)

	// Create client with the transport
	registry := strfmt.NewFormats()
	client := vantage.New(transport, registry)

	return &impl{
		baseURL: baseURL,
		token:   bearerToken,
		client:  client,
	}, nil
}

// mapSettingsToParams maps settings to query parameters.
func mapSettingsToParams(p *costs.GetCostsParams, s *CostSettings) {
	if s.Amortize != nil {
		p.SettingsAmortize = s.Amortize
	}
	if s.IncludeCredits != nil {
		p.SettingsIncludeCredits = s.IncludeCredits
	}
	if s.IncludeRefunds != nil {
		p.SettingsIncludeRefunds = s.IncludeRefunds
	}
	if s.IncludeDiscounts != nil {
		p.SettingsIncludeDiscounts = s.IncludeDiscounts
	}
	if s.IncludeTax != nil {
		p.SettingsIncludeTax = s.IncludeTax
	}
	if s.AggregateBy != nil {
		p.SettingsAggregateBy = s.AggregateBy
	}
	if s.ShowPreviousPeriod != nil {
		p.SettingsShowPreviousPeriod = s.ShowPreviousPeriod
	}
	if s.UnAllocated != nil {
		p.SettingsUnallocated = s.UnAllocated
	}
}

// GetCosts fetches costs from the Vantage API.
// startDate and endDate should be in ISO 8601 format (YYYY-MM-DD).
// Groupings should be comma-separated (e.g., "provider,service,account_id").
func (c *impl) GetCosts(ctx context.Context, params *GetCostsParams) (*CostsResponse, error) {
	if params == nil {
		params = &GetCostsParams{}
	}

	// Build query parameters for the generated client
	p := costs.NewGetCostsParams()
	p.Context = ctx

	// Set authentication and parameters
	if params.CostReportToken != nil {
		p.CostReportToken = params.CostReportToken
	}
	if params.WorkspaceToken != nil {
		p.WorkspaceToken = params.WorkspaceToken
	}
	if params.Filter != nil {
		p.Filter = params.Filter
	}
	if params.StartDate != nil {
		p.StartDate = params.StartDate
	}
	if params.EndDate != nil {
		p.EndDate = params.EndDate
	}
	if params.DateBin != nil {
		p.DateBin = params.DateBin
	}
	if len(params.Groupings) > 0 {
		p.Groupings = params.Groupings
	}
	if params.Page != nil {
		p.Page = params.Page
	}
	if params.Limit != nil {
		p.Limit = params.Limit
	}

	// Map settings if provided
	if params.Settings != nil {
		mapSettingsToParams(p, params.Settings)
	}

	// Call the generated client with nil authInfo (authentication is on the transport)
	result, err := c.client.Costs.GetCosts(p, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get costs from Vantage API: %w", err)
	}

	// Convert generated models to our domain types
	return convertCostsResponse(result.Payload), nil
}

// convertCostsResponse converts generated Costs response to our domain types.
func convertCostsResponse(payload *models.Costs) *CostsResponse {
	resp := &CostsResponse{}

	if payload == nil {
		return resp
	}

	// Convert costs array
	resp.Costs = convertCostRows(payload.Costs)

	// Convert total cost
	if payload.TotalCost != nil {
		resp.TotalCost = &CostAmount{
			Amount:   payload.TotalCost.Amount,
			Currency: payload.TotalCost.Currency,
		}
	}

	// Convert total usage
	resp.TotalUsage = convertUsageAmounts(payload.TotalUsage)

	// Convert links
	if payload.Links != nil {
		resp.Links = &PaginationLinks{
			Self:  payload.Links.Self,
			First: payload.Links.First,
			Next:  payload.Links.Next,
			Prev:  payload.Links.Prev,
			Last:  payload.Links.Last,
		}
	}

	return resp
}

// convertCostRows converts generated Cost models to CostRow domain types.
func convertCostRows(costs []*models.Cost) []*CostRow {
	if len(costs) == 0 {
		return nil
	}

	rows := make([]*CostRow, len(costs))
	for i, c := range costs {
		rows[i] = &CostRow{
			AccruedAt:        c.AccruedAt,
			Amount:           c.Amount,
			Currency:         c.Currency,
			Usage:            c.Usage,
			UsageUnit:        c.UsageUnit,
			Provider:         c.Provider,
			Service:          c.Service,
			AccountID:        c.AccountID,
			BillingAccountID: c.BillingAccountID,
			Region:           c.Region,
			ResourceID:       c.ResourceID,
			ResourceName:     c.ResourceName,
			Tags:             c.Tags,
			CostCategory:     c.CostCategory,
			CostSubcategory:  c.CostSubcategory,
			ChargeType:       c.ChargeType,
			Tagged:           c.Tagged,
			Segment:          c.Segment,
		}
	}
	return rows
}

// convertUsageAmounts converts generated UsagePartial models to UsageAmount domain types.
func convertUsageAmounts(usage []*models.UsagePartial) []*UsageAmount {
	if len(usage) == 0 {
		return nil
	}

	amounts := make([]*UsageAmount, len(usage))
	for i, u := range usage {
		amounts[i] = &UsageAmount{
			Amount: u.Amount,
			Unit:   u.Unit,
		}
	}
	return amounts
}

// isLoopbackHost checks if the given host is a loopback address.
// Handles both bare hostnames and host:port format.
func isLoopbackHost(host string) bool {
	// Check literal loopback names
	if host == "localhost" || strings.HasPrefix(host, "localhost:") {
		return true
	}

	// Strip port if present (host:port format)
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Only strip if the part after : looks like a port number (no more colons)
		afterColon := host[idx+1:]
		if !strings.Contains(afterColon, ":") {
			hostname = host[:idx]
		}
	}

	// Parse as IP and check if loopback
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

// parseBaseURL splits a base URL into host and path.
// Examples:
//   - "https://api.vantage.sh/v2" -> ("api.vantage.sh", "/v2")
//   - "https://api.example.com" -> ("api.example.com", "/")
func parseBaseURL(baseURL string) (string, string) {
	const hostPathSplitCount = 2

	// Remove scheme
	url := strings.TrimPrefix(baseURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split on first slash
	parts := strings.SplitN(url, "/", hostPathSplitCount)
	host := parts[0]
	path := "/"

	if len(parts) > 1 && parts[1] != "" {
		path = "/" + parts[1]
	}

	return host, path
}
