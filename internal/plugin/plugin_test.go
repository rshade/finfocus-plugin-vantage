package plugin

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantageapi"
)

type fakeClient struct {
	response *vantageapi.CostsResponse
	err      error
	params   *vantageapi.GetCostsParams
}

func (f *fakeClient) GetCosts(_ context.Context, p *vantageapi.GetCostsParams) (*vantageapi.CostsResponse, error) {
	f.params = p
	return f.response, f.err
}

func testPlugin(c *fakeClient) *Plugin {
	p := New("test")
	p.costReportToken = "report"
	p.requestInterval = 0
	p.clientFactory = func(string) (vantageapi.Client, error) { return c, nil }
	return p
}

func testRequest() *pbc.GetActualCostRequest {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	return &pbc.GetActualCostRequest{
		ResourceId: "i-123",
		Start:      timestamppb.New(now),
		End:        timestamppb.New(now.Add(24 * time.Hour)),
		Tags:       map[string]string{"provider": "aws", "resource_type": "aws:ec2/instance:Instance"},
	}
}

func TestGetActualCostMapsRowsAndQuery(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "secret")
	c := &fakeClient{
		response: &vantageapi.CostsResponse{
			Costs: []*vantageapi.CostRow{
				{
					Amount:           "12.34",
					Currency:         "USD",
					AccruedAt:        "2026-01-02",
					ResourceID:       strPtr("i-123"),
					Service:          strPtr("Compute"),
					BillingAccountID: strPtr("123456"),
					Usage:            1.0,
					UsageUnit:        strPtr("hour"),
				},
			},
		},
	}
	resp, err := testPlugin(c).GetActualCost(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetResults()) != 1 || resp.GetResults()[0].GetCost() != 12.34 ||
		resp.GetResults()[0].GetSource() != "vantage" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.GetResults()[0].GetFocusRecord() == nil ||
		resp.GetResults()[0].GetFocusRecord().GetBillingCurrency() != "USD" {
		t.Fatalf("expected validated FOCUS record with currency: %+v", resp.GetResults()[0].GetFocusRecord())
	}
	if c.params == nil || c.params.CostReportToken == nil || *c.params.CostReportToken != "report" {
		t.Fatalf("query missing report token: %+v", c.params)
	}
}

func TestGetActualCostRejectsInvalidRequest(t *testing.T) {
	_, err := testPlugin(&fakeClient{}).GetActualCost(context.Background(), &pbc.GetActualCostRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestGetActualCostRejectsMixedCurrencies(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "secret")
	c := &fakeClient{
		response: &vantageapi.CostsResponse{
			Costs: []*vantageapi.CostRow{
				{Amount: "1", Currency: "USD", AccruedAt: "2026-01-02"},
				{Amount: "1", Currency: "EUR", AccruedAt: "2026-01-02"},
			},
		},
	}
	_, err := testPlugin(c).GetActualCost(context.Background(), testRequest())
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %s, want FailedPrecondition (%v)", status.Code(err), err)
	}
}

func TestGetActualCostMapsUpstreamFailure(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "secret")
	_, err := testPlugin(&fakeClient{err: errors.New("unreachable")}).GetActualCost(context.Background(), testRequest())
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %s, want Unavailable (%v)", status.Code(err), err)
	}
}

func TestGetActualCostPaginatesUsingVantagePageLinks(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "secret")
	next := "https://api.vantage.sh/v2/costs?page=2&limit=50"
	c := &fakeClient{response: &vantageapi.CostsResponse{Links: &vantageapi.PaginationLinks{Next: &next}}}
	p := testPlugin(c)
	resp, err := p.GetActualCost(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetNextPageToken() != "2" {
		t.Fatalf("next token = %q, want 2", resp.GetNextPageToken())
	}
	pageToken := resp.GetNextPageToken()
	req := testRequest()
	req.PageToken = pageToken
	if _, callErr := p.GetActualCost(context.Background(), req); callErr != nil {
		t.Fatal(callErr)
	}
	if c.params.Page == nil || *c.params.Page != 2 {
		t.Fatalf("Vantage page = %v, want 2", c.params.Page)
	}
}

func TestGetActualCostUsesPerRequestCredential(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "environment-token")
	c := &fakeClient{response: &vantageapi.CostsResponse{}}
	p := testPlugin(c)
	p.clientFactory = func(token string) (vantageapi.Client, error) {
		if token != "request-token" {
			t.Fatalf("client token = %q, want per-request token", token)
		}
		return c, nil
	}
	credentials, err := pluginsdk.NewCredentials(map[string]string{"vantage-token": "request-token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := pluginsdk.WithCredentials(context.Background(), credentials)
	if _, callErr := p.GetActualCost(ctx, testRequest()); callErr != nil {
		t.Fatal(callErr)
	}
}

func TestSupportsProvider(t *testing.T) {
	p := New("test")
	request := &pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "ec2"}}
	response, err := p.Supports(context.Background(), request)
	if err != nil || !response.GetSupported() {
		t.Fatalf("AWS support = %v, err = %v", response, err)
	}
	response, err = p.Supports(
		context.Background(),
		&pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{Provider: "unknown", ResourceType: "resource"}},
	)
	if err != nil || response.GetSupported() {
		t.Fatalf("unknown support = %v, err = %v", response, err)
	}
}

func TestPluginMetadataAndUnsupportedOperations(t *testing.T) {
	p := New("test")
	if p.Name() != "vantage" {
		t.Fatalf("name = %q", p.Name())
	}
	if _, err := p.GetProjectedCost(context.Background(), nil); status.Code(err) != codes.Unimplemented {
		t.Fatalf("projected status = %v", err)
	}
	if _, err := p.GetPricingSpec(context.Background(), nil); status.Code(err) != codes.Unimplemented {
		t.Fatalf("pricing status = %v", err)
	}
	if _, err := p.EstimateCost(context.Background(), nil); status.Code(err) != codes.Unimplemented {
		t.Fatalf("estimate status = %v", err)
	}
	p.ConsumesPerRequestCredentials()
}

func TestRequestValidationAndMappingErrors(t *testing.T) {
	if err := validateRequest(testRequest()); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	invalid := testRequest()
	invalid.Tags = map[string]string{"provider": "aws", "bad\nkey": "x"}
	if status.Code(validateRequest(invalid)) != codes.InvalidArgument {
		t.Fatal("unsafe label key accepted")
	}
	if _, err := requestPage("0"); err == nil {
		t.Fatal("page zero accepted")
	}
	if _, err := nextPageToken("https://example.test/costs"); err == nil {
		t.Fatal("next link without page accepted")
	}
	_, _, amountErr := mapCostRow(testRequest(), "aws", &vantageapi.CostRow{Amount: "not-money"})
	if status.Code(amountErr) != codes.DataLoss {
		t.Fatalf("amount status = %v", amountErr)
	}
	_, _, dateErr := mapCostRow(testRequest(), "aws", &vantageapi.CostRow{Amount: "1", AccruedAt: "not-a-date"})
	if status.Code(dateErr) != codes.DataLoss {
		t.Fatalf("date status = %v", dateErr)
	}
}

func TestRPCLogIncludesTraceIDFromMetadata(t *testing.T) {
	var logBuffer bytes.Buffer
	p := New("test")
	p.logger = zerolog.New(&logBuffer)
	traceID := "0123456789abcdef0123456789abcdef"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(pluginsdk.TraceIDMetadataKey, traceID))
	_, err := p.Supports(
		ctx,
		&pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "ec2"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	output := logBuffer.String()
	if !strings.Contains(output, `"trace_id":"`+traceID+`"`) {
		t.Fatalf("log omitted trace ID: %s", output)
	}
	if !strings.Contains(output, `"operation":"supports"`) {
		t.Fatalf("log omitted operation: %s", output)
	}
}

func TestVQLQuoteAndProviderFromARN(t *testing.T) {
	if got := vqlQuote("a'b"); got != "a\\'b" {
		t.Fatalf("quote = %q", got)
	}
	if got := providerFromARN("arn:aws:ec2:us-east-1:123:instance/i-1"); got != "aws" {
		t.Fatalf("provider = %q", got)
	}
	if got := providerFromARN("not-an-arn"); got != "" {
		t.Fatalf("provider = %q, want empty", got)
	}
}
