package vantageapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client construction.
func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("empty token returns error", func(t *testing.T) {
		t.Parallel()
		_, err := NewClient("https://api.vantage.sh/v2", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bearer token is required")
	})

	t.Run("valid token succeeds", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("https://api.vantage.sh/v2", "test-token")
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("default base URL is used when empty", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("", "test-token")
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("http allowed on loopback localhost", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8080/v2", "test-token")
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("http allowed on loopback 127.0.0.1", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://127.0.0.1:8080", "test-token")
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("http rejected on non-loopback", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://api.example.com", "test-token")
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "loopback")
	})
}

// TestParseBaseURL tests URL parsing into host and path.
func TestParseBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		wantHost string
		wantPath string
	}{
		{
			name:     "full URL with path",
			baseURL:  "https://api.vantage.sh/v2",
			wantHost: "api.vantage.sh",
			wantPath: "/v2",
		},
		{
			name:     "URL without path",
			baseURL:  "https://api.example.com",
			wantHost: "api.example.com",
			wantPath: "/",
		},
		{
			name:     "URL without scheme",
			baseURL:  "api.vantage.sh/v2",
			wantHost: "api.vantage.sh",
			wantPath: "/v2",
		},
		{
			name:     "URL with port",
			baseURL:  "https://localhost:8080/v2",
			wantHost: "localhost:8080",
			wantPath: "/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, path := parseBaseURL(tt.baseURL)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

// TestGetCosts_WithHttptest verifies the wrapper makes the right HTTP request.
func TestGetCosts_WithHttptest(t *testing.T) {
	t.Parallel()

	// Track the request made by the client
	var (
		capturedMethod  string
		capturedPath    string
		capturedQuery   string
		capturedAuthHdr string
	)

	// Create a mock Vantage API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedAuthHdr = r.Header.Get("Authorization")

		// Respond with a valid Costs structure
		response := map[string]interface{}{
			"costs": []map[string]interface{}{
				{
					"accrued_at":  "2026-10-01T00:00:00Z",
					"amount":      "123.45",
					"currency":    "USD",
					"provider":    "aws",
					"service":     "Amazon Elastic Compute Cloud - Compute",
					"account_id":  "123456789012",
					"region":      "us-east-1",
					"resource_id": "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
					"tags":        []string{"env:prod", "team:platform"},
				},
			},
			"total_cost": map[string]string{
				"amount":   "123.45",
				"currency": "USD",
			},
			"total_usage": []map[string]string{
				{
					"amount": "100",
					"unit":   "hours",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client pointing to mock server
	client, err := NewClient(server.URL, "test-bearer-token")
	require.NoError(t, err)

	// Call GetCosts with specific parameters
	ctx := context.Background()
	params := &GetCostsParams{
		CostReportToken: strPtr("cr_test123"),
		StartDate:       strPtr("2026-10-01"),
		EndDate:         strPtr("2026-10-31"),
		DateBin:         strPtr("day"),
		Groupings:       []string{"provider", "service", "account_id", "region"},
		Limit:           int32Ptr(100),
	}

	result, err := client.GetCosts(ctx, params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify the request was made correctly
	assert.Equal(t, "GET", capturedMethod)
	assert.Equal(t, "/costs", capturedPath)
	assert.Contains(t, capturedQuery, "cost_report_token=cr_test123")
	assert.Contains(t, capturedQuery, "start_date=2026-10-01")
	assert.Contains(t, capturedQuery, "end_date=2026-10-31")
	assert.Contains(t, capturedQuery, "date_bin=day")
	assert.Contains(t, capturedQuery, "groupings=provider%2Cservice%2Caccount_id%2Cregion")
	assert.Contains(t, capturedQuery, "limit=100")

	// Verify bearer token was set
	assert.Equal(t, "Bearer test-bearer-token", capturedAuthHdr)

	// Verify response was parsed
	require.NotNil(t, result.TotalCost)
	assert.Equal(t, "123.45", result.TotalCost.Amount)
	assert.Equal(t, "USD", result.TotalCost.Currency)

	require.Len(t, result.Costs, 1)
	assert.Equal(t, "2026-10-01T00:00:00Z", result.Costs[0].AccruedAt)
	assert.Equal(t, "123.45", result.Costs[0].Amount)
	assert.Equal(t, "aws", *result.Costs[0].Provider)
	assert.Equal(t, "us-east-1", *result.Costs[0].Region)
	assert.Len(t, result.Costs[0].Tags, 2)
	assert.Equal(t, "env:prod", result.Costs[0].Tags[0])
}

// TestGetCosts_BreakCheck verifies that changing the parameter name breaks the test.
// This ensures the test actually validates the request format.
func TestGetCosts_BreakCheck(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If this test passes, it means the parameter was sent.
		// To trigger the break check, comment out the line in client.go that sets the parameter.
		if !strings.Contains(r.URL.RawQuery, "limit=50") {
			http.Error(w, "limit parameter not found in query", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"costs":       []interface{}{},
			"total_cost":  map[string]string{"amount": "0", "currency": "USD"},
			"total_usage": []interface{}{},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	require.NoError(t, err)

	params := &GetCostsParams{
		Limit: int32Ptr(50),
	}

	result, err := client.GetCosts(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestGetCosts_EmptyParams uses default parameters.
func TestGetCosts_EmptyParams(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"costs":       []interface{}{},
			"total_cost":  map[string]string{"amount": "0", "currency": "USD"},
			"total_usage": []interface{}{},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	require.NoError(t, err)

	// Call with nil params (should use defaults)
	result, err := client.GetCosts(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Costs)
}

// TestGetCosts_Settings tests that cost settings are passed correctly.
func TestGetCosts_Settings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify settings were sent in the query string
		if !strings.Contains(r.URL.RawQuery, "settings%5Bamortize%5D=true") ||
			!strings.Contains(r.URL.RawQuery, "settings%5Binclude_tax%5D=true") {
			http.Error(w, "settings not found in query", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"costs":       []interface{}{},
			"total_cost":  map[string]string{"amount": "0", "currency": "USD"},
			"total_usage": []interface{}{},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	require.NoError(t, err)

	params := &GetCostsParams{
		Settings: &CostSettings{
			Amortize:   boolPtr(true),
			IncludeTax: boolPtr(true),
		},
	}

	result, err := client.GetCosts(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Helper functions for test param construction.

func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
