package plugin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGetActualCostAgainstMockVantageAPI(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "environment-token")
	t.Setenv("FINFOCUS_VANTAGE_COST_REPORT_TOKEN", "report-token")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/costs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer request-token" {
			t.Errorf("per-request auth not forwarded")
		}
		query := r.URL.Query()
		if query.Get("cost_report_token") != "report-token" {
			t.Errorf("missing report selector")
		}
		wantFilter := "costs.provider = 'aws' AND costs.service = 'Amazon Elastic Compute Cloud - Compute' AND costs.resource_id = 'i-123'"
		if query.Get("filter") != wantFilter {
			t.Errorf("filter = %q, want %q", query.Get("filter"), wantFilter)
		}
		if query.Get("settings[amortize]") != "false" || query.Get("settings[include_credits]") != "true" {
			t.Errorf("wrong cost basis: %v", query)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		page := query.Get("page")
		if page != "1" && page != "2" {
			t.Errorf("unexpected page: %q", page)
		}
		fixture, err := os.ReadFile("testdata/costs_page" + page + ".json")
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()
	t.Setenv("FINFOCUS_VANTAGE_BASE_URL", server.URL+"/v2")
	p := New("test")
	p.requestInterval = 0
	client := serveTestPlugin(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-finfocus-credential-vantage-token", "request-token")
	req := testRequest()
	resp, err := client.GetActualCost(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	type mappedCost struct {
		Cost      float64 `json:"cost"`
		Currency  string  `json:"currency"`
		Account   string  `json:"account"`
		Resource  string  `json:"resource"`
		Usage     float64 `json:"usage"`
		Timestamp string  `json:"timestamp"`
	}
	var actual []mappedCost
	for _, item := range resp.GetResults() {
		focus := item.GetFocusRecord()
		if focus == nil {
			t.Fatal("missing validated FOCUS record")
		}
		actual = append(
			actual,
			mappedCost{
				item.GetCost(),
				focus.GetBillingCurrency(),
				focus.GetBillingAccountId(),
				focus.GetResourceId(),
				focus.GetConsumedQuantity(),
				item.GetTimestamp().AsTime().Format(time.RFC3339),
			},
		)
	}
	golden, err := os.ReadFile("testdata/expected_costs.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []mappedCost
	if err = json.Unmarshal(golden, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("mapped costs = %+v, want %+v", actual, expected)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d, want one retry and two pages", calls.Load())
	}
	if resp.GetNextPageToken() != "" || req.GetPageToken() != "" {
		t.Fatal("default query omitted pages or mutated request")
	}
}

func serveTestPlugin(t *testing.T, p *Plugin) pbc.CostSourceServiceClient {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- pluginsdk.Serve(ctx, pluginsdk.ServeConfig{
			Plugin: p, Listener: listener,
			PluginInfo: &pluginsdk.PluginInfo{Name: "finfocus-plugin-vantage", Version: "test", SpecVersion: pluginsdk.SpecVersion},
		})
	}()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		cancel()
		select {
		case serveErr := <-done:
			if serveErr != nil {
				t.Errorf("server shutdown: %v", serveErr)
			}
		case <-time.After(3 * time.Second):
			t.Error("gRPC server did not stop")
		}
	})
	return pbc.NewCostSourceServiceClient(conn)
}

func TestGRPCCredentialMetadataAndErrors(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "")
	t.Setenv("FINFOCUS_VANTAGE_COST_REPORT_TOKEN", "report")
	client := serveTestPlugin(t, New("test"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := client.GetPluginInfo(ctx, &pbc.GetPluginInfoRequest{})
	if err != nil || info.GetMetadata()["supports_per_request_credentials"] != "true" {
		t.Fatalf("metadata=%v, err=%v", info, err)
	}
	if _, err = client.GetActualCost(ctx, testRequest()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing credential: %v", err)
	}
	bad := metadata.AppendToOutgoingContext(ctx, "x-finfocus-credential-vantage-token", strings.Repeat("x", 4097))
	if _, err = client.GetActualCost(bad, testRequest()); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("oversized credential: %v", err)
	}
}
