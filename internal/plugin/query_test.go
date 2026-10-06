package plugin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantageapi"
)

func TestResourceDescriptorBuildsDocumentedVQL(t *testing.T) {
	req := testRequest()
	req.Resource = &pbc.ResourceDescriptor{
		Provider:     "aws",
		ResourceType: "aws:ec2/instance:Instance",
		Region:       "us-east-1",
	}
	req.Arn = "arn:aws:ec2:us-east-1:123:instance/i-123"
	req.Tags = map[string]string{"team": "O'Reilly\\finance", "provider": "cloud-tag", "user:env": "prod"}
	want := "costs.provider = 'aws' AND costs.service = 'Amazon Elastic Compute Cloud - Compute' AND " +
		"costs.resource_id = 'arn:aws:ec2:us-east-1:123:instance/i-123' AND costs.region = 'us-east-1' AND " +
		"(tags.name, tags.value) IN (('provider', 'cloud-tag'), ('team', 'O\\'Reilly\\\\finance'), ('user:env', 'prod'))"
	if got := buildFilter(req, requestProvider(req)); got != want {
		t.Fatalf("filter = %q, want %q", got, want)
	}
}

func TestUnknownServiceIsRejectedBeforeAPI(t *testing.T) {
	req := testRequest()
	req.Tags = map[string]string{"provider": "gcp"}
	client := &fakeClient{}
	_, err := testPlugin(client).GetActualCost(context.Background(), req)
	if status.Code(err) != codes.InvalidArgument || client.params != nil {
		t.Fatalf("missing service: err=%v, query=%v", err, client.params)
	}
}

func TestUpstreamStatusesAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		code int
		want codes.Code
	}{
		{http.StatusUnauthorized, codes.Unauthenticated},
		{http.StatusForbidden, codes.PermissionDenied},
		{http.StatusBadRequest, codes.InvalidArgument},
		{http.StatusUnprocessableEntity, codes.InvalidArgument},
		{http.StatusNotFound, codes.NotFound},
		{http.StatusPaymentRequired, codes.FailedPrecondition},
		{http.StatusTooManyRequests, codes.Unavailable},
		{http.StatusServiceUnavailable, codes.Unavailable},
	} {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			err := upstreamStatus(&vantageapi.APIError{StatusCode: tc.code})
			if status.Code(err) != tc.want {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	if err := upstreamStatus(errors.New("token=secret")); strings.Contains(err.Error(), "secret") {
		t.Fatalf("upstream error leaked secret: %v", err)
	}
	if status.Code(upstreamStatus(context.Canceled)) != codes.Canceled ||
		status.Code(upstreamStatus(context.DeadlineExceeded)) != codes.DeadlineExceeded {
		t.Fatal("context status was lost")
	}
}

func TestCanceledRequestDoesNotCallAPI(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "test")
	client := &fakeClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testPlugin(client).GetActualCost(ctx, testRequest())
	if status.Code(err) != codes.Canceled || client.params != nil {
		t.Fatalf("canceled request: err=%v, query=%v", err, client.params)
	}
}

func TestMapCostsHonorsWindowAndAccountOverride(t *testing.T) {
	req := testRequest()
	req.BillingAccountId = "caller-account"
	response := &vantageapi.CostsResponse{Costs: []*vantageapi.CostRow{
		{
			Amount:           "5",
			Currency:         "USD",
			AccruedAt:        "2026-01-02",
			BillingAccountID: strPtr("source-account"),
			Service:          strPtr("EC2"),
			Usage:            1,
			UsageUnit:        strPtr("hour"),
		},
		{Amount: "100", Currency: "USD", AccruedAt: "2026-01-03"},
	}}
	got, err := mapCosts(req, "aws", response)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GetResults()) != 1 || got.GetResults()[0].GetFocusRecord().GetBillingAccountId() != "caller-account" {
		t.Fatalf("unexpected mapped results: %v", got)
	}
	if got.GetTotalCount() != 0 {
		t.Fatal("page length was advertised as whole-query count")
	}
	if _, nilErr := mapCosts(req, "aws", nil); status.Code(nilErr) != codes.DataLoss {
		t.Fatal("nil response accepted")
	}
}

func TestRequestThrottleIsCancelable(t *testing.T) {
	p := New("test")
	if err := p.waitForRequest(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := p.waitForRequest(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait returned %v", err)
	}
}
