package vantageapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorPreservesStatusAndRetryWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":["secret-token must not escape"]}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetCosts(context.Background(), nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests ||
		apiErr.RetryAfter != 2*time.Second {
		t.Fatalf("unexpected API error: %v", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatal("error exposed response body")
	}
}

func TestRetryHeadersUseUnixResetAndCap(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, tc := range []struct {
		name, retry, reset string
		want               time.Duration
	}{
		{"unix reset", "", "1700000005", 5 * time.Second},
		{"retry seconds", "3", "", 3 * time.Second},
		{"retry date", now.Add(4 * time.Second).UTC().Format(http.TimeFormat), "", 4 * time.Second},
		{"past reset", "", "1699999999", 0},
		{"cap", "9223372036854775807", "", time.Minute},
		{"bad headers", "invalid", "invalid", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("Retry-After", tc.retry)
			headers.Set("X-Rate-Limit-Reset", tc.reset)
			if got := retryDelay(headers, now); got != tc.want {
				t.Fatalf("delay=%v, want %v", got, tc.want)
			}
		})
	}
}
