package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantageapi"
)

func TestGetActualCostAgainstMockVantageAPI(t *testing.T) {
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "integration-token")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/costs" {
			t.Errorf("path = %q, want /v2/costs", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer integration-token" {
			t.Errorf("authorization = %q", got)
		}
		if r.URL.Query().Get("cost_report_token") != "report-token" {
			t.Errorf("missing cost report selector")
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"costs":[{"amount":"4.25","currency":"USD","accrued_at":"2026-01-02","provider":"aws","service":"EC2","account_id":"123","billing_account_id":"123","usage":1,"usage_unit":"hour","resource_id":"i-123","tags":[]}],"total_cost":{"amount":"4.25","currency":"USD"},"total_usage":[]}`,
			),
		)
	}))
	defer server.Close()
	p := New("test")
	p.costReportToken = "report-token"
	p.clientFactory = func(token string) (vantageapi.Client, error) { return vantageapi.NewClient(server.URL+"/v2", token) }
	resp, err := p.GetActualCost(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(resp.GetResults()); got != 1 {
		t.Fatalf("results = %d, want 1", got)
	}
	if got := resp.GetResults()[0].GetCost(); got != 4.25 {
		t.Fatalf("cost = %v, want 4.25", got)
	}
	if resp.GetResults()[0].GetFocusRecord() == nil {
		t.Fatal("expected validated FOCUS record")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("Vantage calls = %d, want 2 including retry", got)
	}
}
