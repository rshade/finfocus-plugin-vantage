package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				BaseURL:    "https://api.vantage.sh",
				Token:      "test-token",
				Timeout:    time.Second * 30,
				MaxRetries: 3,
				Logger:     NewNoopLogger(),
			},
			wantErr: false,
		},
		{
			name: "missing token",
			config: Config{
				BaseURL: "https://api.vantage.sh",
				Logger:  NewNoopLogger(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_Costs(t *testing.T) {
	// Mock server response.
	mockResponse := CostsResponse{
		Data: []CostRow{
			{
				Provider:    "aws",
				Service:     "EC2",
				Cost:        100.50,
				Currency:    "USD",
				BucketStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
		},
		NextCursor: "next-page-cursor",
		HasMore:    true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request.
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Check query parameters.
		assert.Equal(t, "test-workspace", r.URL.Query().Get("workspace_token"))
		assert.Equal(t, "day", r.URL.Query().Get("granularity"))
		assert.Contains(t, r.URL.Query()["group_bys[]"], "provider")
		assert.Contains(t, r.URL.Query()["metrics[]"], "cost")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
		GroupBys:       []string{"provider", "service"},
		Metrics:        []string{"cost", "usage"},
		PageSize:       100,
	}

	page, err := client.Costs(context.Background(), query)
	require.NoError(t, err)

	assert.Len(t, page.Data, 1)
	assert.Equal(t, "aws", page.Data[0].Provider)
	assert.Equal(t, "EC2", page.Data[0].Service)
	assert.InEpsilon(t, 100.50, page.Data[0].Cost, 0.01)
	assert.Equal(t, "next-page-cursor", page.NextCursor)
	assert.True(t, page.HasMore)
}

func TestClient_Forecast(t *testing.T) {
	// Mock server response.
	mockResponse := ForecastResponse{
		Data: []ForecastRow{
			{
				BucketStart: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				Cost:        150.75,
				Currency:    "USD",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request.
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/cost_reports/test-report-token/forecast", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	forecast, err := client.Forecast(context.Background(), "test-report-token", query)
	require.NoError(t, err)

	assert.Len(t, forecast.Data, 1)
	assert.InEpsilon(t, 150.75, forecast.Data[0].Cost, 0.01)
	assert.Equal(t, "USD", forecast.Data[0].Currency)
}

func TestClient_RetryOn5xx(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			// First call fails with 503.
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Second call succeeds.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount) // Should have retried once
}

func TestClient_RateLimitHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ratelimit-Reset", "60") // Reset in 60 seconds
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0, // Don't retry to avoid waiting
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.Error(t, err)
	// Should be a rate limit error.
	assert.Contains(t, err.Error(), "rate limited")
}

func TestPager_NextPage(t *testing.T) {
	// First page response.
	firstResponse := CostsResponse{
		Data: []CostRow{
			{Provider: "aws", Cost: 100},
		},
		NextCursor: "cursor-2",
		HasMore:    true,
	}

	// Second page response.
	secondResponse := CostsResponse{
		Data: []CostRow{
			{Provider: "gcp", Cost: 200},
		},
		NextCursor: "",
		HasMore:    false,
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(firstResponse)
		} else {
			_ = json.NewEncoder(w).Encode(secondResponse)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	pager := NewPager(client, Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}, NewNoopLogger())

	// First page.
	page1, err := pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.Len(t, page1.Data, 1)
	assert.Equal(t, "aws", page1.Data[0].Provider)
	assert.Equal(t, "cursor-2", page1.NextCursor)
	assert.True(t, page1.HasMore)

	// Second page.
	page2, err := pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.Len(t, page2.Data, 1)
	assert.Equal(t, "gcp", page2.Data[0].Provider)
	assert.Empty(t, page2.NextCursor)
	assert.False(t, page2.HasMore)

	assert.Equal(t, 2, callCount)

	// Third attempt should fail since there are no more pages.
	_, err = pager.NextPage(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no more pages available")
}

func TestPager_NextPage_ClientError(t *testing.T) {
	// Mock server that returns an error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	pager := NewPager(client, Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}, NewNoopLogger())

	// NextPage should fail due to API error.
	_, err = pager.NextPage(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching costs page")
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("test-token")

	assert.Equal(t, "https://api.vantage.sh", config.BaseURL)
	assert.Equal(t, "test-token", config.Token)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, 5, config.MaxRetries)
	assert.NotNil(t, config.Logger)
}

func TestPager_HasMore(t *testing.T) {
	client, err := New(Config{
		BaseURL:    "https://api.vantage.sh",
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	// Test with empty cursor (should return false initially).
	pager := NewPager(client, Query{}, NewNoopLogger())
	assert.False(t, pager.HasMore())

	// Test with cursor (should return true).
	pager.query.Cursor = "test-cursor"
	assert.True(t, pager.HasMore())
}

func TestPager_AllPages(t *testing.T) {
	// Mock server response with multiple pages.
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First page.
			resp := CostsResponse{
				Data: []CostRow{
					{Provider: "aws", Cost: 100},
				},
				NextCursor: "cursor-2",
				HasMore:    true,
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
		} else {
			// Second page (final).
			resp := CostsResponse{
				Data: []CostRow{
					{Provider: "gcp", Cost: 200},
				},
				NextCursor: "",
				HasMore:    false,
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
		}
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	pager := NewPager(client, Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}, NewNoopLogger())

	rows, err := pager.AllPages(context.Background())
	require.NoError(t, err)

	assert.Len(t, rows, 2)
	assert.Equal(t, "aws", rows[0].Provider)
	assert.InEpsilon(t, 100.0, rows[0].Cost, 0.01)
	assert.Equal(t, "gcp", rows[1].Provider)
	assert.InEpsilon(t, 200.0, rows[1].Cost, 0.01)
	assert.Equal(t, 2, callCount)
}

func TestClient_ForecastRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			// First call fails with 503.
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Second call succeeds.
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ForecastResponse{Data: []ForecastRow{}}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	_, err = client.Forecast(context.Background(), "test-report-token", query)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount) // Should have retried once
}

func TestClient_ForecastEmptyResponse(t *testing.T) {
	// Test that empty forecast data is handled gracefully.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/cost_reports/test-report-token/forecast", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// Return empty data array.
		_ = json.NewEncoder(w).Encode(ForecastResponse{Data: []ForecastRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	forecast, err := client.Forecast(context.Background(), "test-report-token", query)
	require.NoError(t, err)
	assert.Empty(t, forecast.Data) // Should handle empty data gracefully
}

func TestClient_ForecastNotFound(t *testing.T) {
	// Test handling of 404 error when forecast report doesn't exist.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/forecast")

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "Cost report not found"}`))
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	_, err = client.Forecast(context.Background(), "nonexistent-report-token", query)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_ForecastMissingReportToken(t *testing.T) {
	// Test handling of empty report token.
	client, err := New(Config{
		BaseURL:    "https://api.vantage.sh",
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	// Empty report token should result in a malformed URL error.
	_, err = client.Forecast(context.Background(), "", query)
	require.Error(t, err)
}

func TestClient_ForecastContextCancellation(t *testing.T) {
	// Test that context cancellation is properly handled.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a slow response that will be cancelled.
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ForecastResponse{Data: []ForecastRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 10,
		MaxRetries: 0,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	// Create context that will be cancelled immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.Forecast(ctx, "test-report-token", query)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestRedactURL(t *testing.T) {
	// Create a test HTTP client directly.
	httpClient := &httpClient{
		token:  "secret-token",
		logger: NewNoopLogger(),
	}

	// Test URL without sensitive data.
	originalURL := "https://api.vantage.sh/costs?param=value"
	redacted := httpClient.redactURL(originalURL)
	assert.Equal(t, originalURL, redacted)

	// Test URL with Authorization parameter.
	urlWithAuth := "https://api.vantage.sh/costs?Authorization=Bearer%20secret-token&param=value"
	redacted = httpClient.redactURL(urlWithAuth)
	expected := "https://api.vantage.sh/costs?Authorization=Bearer%20****&param=value"
	assert.Equal(t, expected, redacted)
}

func TestClient_ContextCancellationDuringRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Always return 503 to trigger retries.
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 30,
		MaxRetries: 5,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	// Create context that cancels after short delay.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	_, err = client.Costs(ctx, query)
	elapsed := time.Since(startTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	// Should fail quickly due to context cancellation, not wait for all retries.
	assert.Less(t, elapsed, time.Second)
}

func TestClient_RetryOn502(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestClient_RetryOn504(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestClient_RetryOn500(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestClient_MaxRetriesExhausted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// Always fail with 503.
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 5,
		MaxRetries: 2,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	_, err = client.Costs(context.Background(), query)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Equal(t, 3, callCount) // Initial + 2 retries
}

func TestClient_ExponentialBackoffTiming(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 30,
		MaxRetries: 2,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	startTime := time.Now()
	_, err = client.Costs(context.Background(), query)
	elapsed := time.Since(startTime)

	require.NoError(t, err)
	assert.Equal(t, 3, callCount)
	// Should have delays between retries (exponential backoff).
	// First retry delay: ~1s, second retry delay: ~2s (with jitter ±25%).
	// Total should be at least 2 seconds but less than 6 seconds.
	assert.GreaterOrEqual(t, elapsed, time.Second*2)
	assert.Less(t, elapsed, time.Second*6)
}

func TestClient_JitterVariance(t *testing.T) {
	// Run multiple retry scenarios and verify jitter adds variance.
	delays := make([]time.Duration, 5)

	for i := range 5 {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
		}))

		client, err := New(Config{
			BaseURL:    server.URL,
			Token:      "test-token",
			Timeout:    time.Second * 10,
			MaxRetries: 1,
			Logger:     NewNoopLogger(),
		})
		require.NoError(t, err)

		query := Query{
			WorkspaceToken: "test-workspace",
			StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Granularity:    "day",
		}

		startTime := time.Now()
		_, err = client.Costs(context.Background(), query)
		delays[i] = time.Since(startTime)

		require.NoError(t, err)
		server.Close()
	}

	// Verify variance in delays (jitter should cause different timings).
	// At least one delay should differ by more than 100ms from another.
	hasVariance := false
	for i := range delays {
		for j := i + 1; j < len(delays); j++ {
			diff := delays[i] - delays[j]
			if diff < 0 {
				diff = -diff
			}
			if diff > 100*time.Millisecond {
				hasVariance = true
				break
			}
		}
		if hasVariance {
			break
		}
	}
	assert.True(t, hasVariance, "jitter should introduce timing variance")
}

func TestClient_RateLimitRetryAfterHeader(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 10,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	startTime := time.Now()
	_, err = client.Costs(context.Background(), query)
	elapsed := time.Since(startTime)

	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	// Should wait at least 1 second due to Retry-After header.
	assert.GreaterOrEqual(t, elapsed, time.Second)
}

func TestClient_429WithSuccessfulBackoff(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("X-RateLimit-Reset", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostsResponse{Data: []CostRow{}})
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:    server.URL,
		Token:      "test-token",
		Timeout:    time.Second * 10,
		MaxRetries: 1,
		Logger:     NewNoopLogger(),
	})
	require.NoError(t, err)

	query := Query{
		WorkspaceToken: "test-workspace",
		StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:    "day",
	}

	startTime := time.Now()
	_, err = client.Costs(context.Background(), query)
	elapsed := time.Since(startTime)

	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	// Should wait at least 1 second.
	assert.GreaterOrEqual(t, elapsed, time.Second)
}

func TestClient_NonRetryable4xxErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				callCount++
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client, err := New(Config{
				BaseURL:    server.URL,
				Token:      "test-token",
				Timeout:    time.Second * 5,
				MaxRetries: 3,
				Logger:     NewNoopLogger(),
			})
			require.NoError(t, err)

			query := Query{
				WorkspaceToken: "test-workspace",
				StartAt:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndAt:          time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				Granularity:    "day",
			}

			_, err = client.Costs(context.Background(), query)
			require.Error(t, err)
			// Should NOT retry on 4xx errors (except 429).
			assert.Equal(t, 1, callCount)
		})
	}
}

// Example usage demonstration.
//
//nolint:testableexamples // Example requires real API credentials
func ExampleNew() {
	// Create a client with default configuration.
	client, err := New(DefaultConfig("your-api-token"))
	if err != nil {
		panic(err)
	}

	// Use the client to fetch costs.
	query := Query{
		CostReportToken: "cr_your_report_token",
		StartAt:         time.Now().AddDate(0, -1, 0), // 1 month ago
		EndAt:           time.Now(),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost", "usage"},
	}

	page, err := client.Costs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	// Process the results.
	for _, row := range page.Data {
		fmt.Printf("Provider: %s, Cost: %.2f\n", row.Provider, row.Cost)
	}
}

//nolint:testableexamples // Example requires real API credentials
func ExampleClient_Forecast() {
	client, err := New(DefaultConfig("your-api-token"))
	if err != nil {
		panic(err)
	}

	// Fetch forecast data.
	query := ForecastQuery{
		StartAt:     time.Now(),
		EndAt:       time.Now().AddDate(0, 3, 0), // 3 months ahead
		Granularity: "month",
	}

	forecast, err := client.Forecast(context.Background(), "cr_your_report_token", query)
	if err != nil {
		panic(err)
	}

	// Process forecast results.
	for _, row := range forecast.Data {
		fmt.Printf("Forecast cost: %.2f at %s\n", row.Cost, row.BucketStart.Format("2006-01-02"))
	}
}
