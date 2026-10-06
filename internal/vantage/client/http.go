package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	exponentialBase  = 2.0
	jitterFraction   = 0.5
	maxBackoffDelay  = 30 * time.Second
	baseBackoffDelay = 1 * time.Second
	jitterOffset     = 0.25

	// Log field constants for structured logging.
	logFieldAdapter   = "adapter"
	logFieldOperation = "operation"
	logFieldAttempt   = "attempt"
	logFieldRows      = "rows"

	// Log operation names.
	logAdapterVantage = "vantage"
	logOpCostsRequest = "costs_request"
	logOpForecastReq  = "forecast_request"
)

// httpClient handles low-level HTTP operations with retry and rate limiting.
type httpClient struct {
	baseURL    string
	token      string
	timeout    time.Duration
	maxRetries int
	logger     Logger
	httpClient *http.Client
}

// newHTTPClient creates a new HTTP client.
func newHTTPClient(config Config) *httpClient {
	return &httpClient{
		baseURL:    strings.TrimSuffix(config.BaseURL, "/"),
		token:      config.Token,
		timeout:    config.Timeout,
		maxRetries: config.MaxRetries,
		logger:     config.Logger,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// doCostsRequest performs a costs API request with retry logic.
func (c *httpClient) doCostsRequest(ctx context.Context, query Query) (Page, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Info(ctx, "Retrying costs request", map[string]interface{}{
				logFieldAdapter:   logAdapterVantage,
				logFieldOperation: logOpCostsRequest,
				logFieldAttempt:   attempt,
				"max_retries":     c.maxRetries,
			})
		}

		page, err := c.doCostsRequestOnce(ctx, query)
		if err == nil {
			if attempt > 0 {
				c.logger.Info(ctx, "Costs request succeeded after retry", map[string]interface{}{
					logFieldAdapter:   logAdapterVantage,
					logFieldOperation: logOpCostsRequest,
					logFieldAttempt:   attempt,
				})
			}
			return page, nil
		}

		lastErr = err

		// Check if we should retry.
		if !c.shouldRetry(err, attempt) {
			break
		}

		// Wait before retrying.
		if waitErr := c.waitBeforeRetry(ctx, attempt, err); waitErr != nil {
			return Page{}, waitErr
		}
	}

	return Page{}, fmt.Errorf("costs request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// doCostsRequestOnce performs a single costs API request.
func (c *httpClient) doCostsRequestOnce(ctx context.Context, query Query) (Page, error) {
	u, err := url.Parse(c.baseURL + "/costs")
	if err != nil {
		return Page{}, fmt.Errorf("parsing URL: %w", err)
	}

	// Build query parameters.
	q := url.Values{}
	if query.WorkspaceToken != "" {
		q.Set("workspace_token", query.WorkspaceToken)
	}
	if query.CostReportToken != "" {
		q.Set("cost_report_token", query.CostReportToken)
	}
	q.Set("start_at", query.StartAt.Format(time.RFC3339))
	q.Set("end_at", query.EndAt.Format(time.RFC3339))
	q.Set("granularity", query.Granularity)

	for _, gb := range query.GroupBys {
		q.Add("group_bys[]", gb)
	}
	for _, m := range query.Metrics {
		q.Add("metrics[]", m)
	}

	if query.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(query.PageSize))
	}
	if query.Cursor != "" {
		q.Set("cursor", query.Cursor)
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Page{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finfocus-vantage/1.0")

	c.logger.Debug(ctx, "Making costs request", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpCostsRequest,
		logFieldAttempt:   0,
		"url":             c.redactURL(u.String()),
		"method":          "GET",
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle rate limiting.
	if resp.StatusCode == http.StatusTooManyRequests {
		resetTime := c.parseRateLimitReset(ctx, resp)
		if resetTime > 0 {
			c.logger.Warn(ctx, "Rate limited, waiting for reset", map[string]interface{}{
				logFieldAdapter:   logAdapterVantage,
				logFieldOperation: logOpCostsRequest,
				logFieldAttempt:   0,
				"reset_in":        time.Duration(resetTime) * time.Second,
			})
			return Page{}, &rateLimitError{resetIn: time.Duration(resetTime) * time.Second}
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error(ctx, "Costs request failed", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: logOpCostsRequest,
			logFieldAttempt:   0,
			"status_code":     resp.StatusCode,
			"response":        string(body),
		})
		return Page{}, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var costsResp CostsResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&costsResp); decodeErr != nil {
		return Page{}, fmt.Errorf("decoding response: %w", decodeErr)
	}

	page := Page(costsResp)

	c.logger.Debug(ctx, "Costs response received", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpCostsRequest,
		logFieldAttempt:   0,
		logFieldRows:      len(page.Data),
		"next_cursor":     page.NextCursor,
		"has_more":        page.HasMore,
	})

	return page, nil
}

// doForecastRequest performs a forecast API request.
func (c *httpClient) doForecastRequest(ctx context.Context, reportToken string, query ForecastQuery) (Forecast, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Info(ctx, "Retrying forecast request", map[string]interface{}{
				logFieldAdapter:   logAdapterVantage,
				logFieldOperation: logOpForecastReq,
				logFieldAttempt:   attempt,
				"max_retries":     c.maxRetries,
			})
		}

		forecast, err := c.doForecastRequestOnce(ctx, reportToken, query)
		if err == nil {
			if attempt > 0 {
				c.logger.Info(ctx, "Forecast request succeeded after retry", map[string]interface{}{
					logFieldAdapter:   logAdapterVantage,
					logFieldOperation: logOpForecastReq,
					logFieldAttempt:   attempt,
				})
			}
			return forecast, nil
		}

		lastErr = err

		// Check if we should retry.
		if !c.shouldRetry(err, attempt) {
			break
		}

		// Wait before retrying.
		if waitErr := c.waitBeforeRetry(ctx, attempt, err); waitErr != nil {
			return Forecast{}, waitErr
		}
	}

	return Forecast{}, fmt.Errorf("forecast request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// doForecastRequestOnce performs a single forecast API request.
func (c *httpClient) doForecastRequestOnce(
	ctx context.Context,
	reportToken string,
	query ForecastQuery,
) (Forecast, error) {
	u, err := url.Parse(fmt.Sprintf("%s/cost_reports/%s/forecast", c.baseURL, reportToken))
	if err != nil {
		return Forecast{}, fmt.Errorf("parsing URL: %w", err)
	}

	// Build query parameters.
	q := url.Values{}
	q.Set("start_at", query.StartAt.Format(time.RFC3339))
	q.Set("end_at", query.EndAt.Format(time.RFC3339))
	q.Set("granularity", query.Granularity)

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Forecast{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finfocus-vantage/1.0")

	c.logger.Debug(ctx, "Making forecast request", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpForecastReq,
		logFieldAttempt:   0,
		"url":             c.redactURL(u.String()),
		"method":          "GET",
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Forecast{}, fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle rate limiting.
	if resp.StatusCode == http.StatusTooManyRequests {
		resetTime := c.parseRateLimitReset(ctx, resp)
		if resetTime > 0 {
			c.logger.Warn(ctx, "Rate limited, waiting for reset", map[string]interface{}{
				logFieldAdapter:   logAdapterVantage,
				logFieldOperation: logOpForecastReq,
				logFieldAttempt:   0,
				"reset_in":        time.Duration(resetTime) * time.Second,
			})
			return Forecast{}, &rateLimitError{resetIn: time.Duration(resetTime) * time.Second}
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error(ctx, "Forecast request failed", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: logOpForecastReq,
			logFieldAttempt:   0,
			"status_code":     resp.StatusCode,
			"response":        string(body),
		})
		return Forecast{}, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var forecastResp ForecastResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&forecastResp); decodeErr != nil {
		return Forecast{}, fmt.Errorf("decoding response: %w", decodeErr)
	}

	forecast := Forecast(forecastResp)

	c.logger.Debug(ctx, "Forecast response received", map[string]interface{}{
		logFieldAdapter:   logAdapterVantage,
		logFieldOperation: logOpForecastReq,
		logFieldAttempt:   0,
		logFieldRows:      len(forecast.Data),
	})

	return forecast, nil
}

// shouldRetry determines if an error should trigger a retry.
func (c *httpClient) shouldRetry(err error, attempt int) bool {
	// Always check attempt count first, regardless of error type.
	if attempt >= c.maxRetries {
		return false
	}

	// Check if this is a retryable error type.
	var rateLimitErr *rateLimitError
	if errors.As(err, &rateLimitErr) {
		return true
	}

	// Retry on 5xx errors and network errors.
	errStr := err.Error()
	return strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "500")
}

// waitBeforeRetry implements exponential backoff with jitter.
func (c *httpClient) waitBeforeRetry(ctx context.Context, attempt int, lastErr error) error {
	var delay time.Duration

	// Check if this is a rate limit error with specific reset time.
	var rateLimitErr *rateLimitError
	if errors.As(lastErr, &rateLimitErr) && rateLimitErr.resetIn > 0 {
		delay = rateLimitErr.resetIn
		c.logger.Debug(ctx, "Waiting for rate limit reset", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "rate_limit_wait",
			logFieldAttempt:   attempt,
			"delay":           delay,
		})
	} else {
		// Exponential backoff: baseDelay * exponentialBase^attempt.
		delay = time.Duration(float64(baseBackoffDelay) * math.Pow(exponentialBase, float64(attempt)))

		// Add jitter (±25%) as a fraction.
		//nolint:gosec // math/rand/v2 is acceptable for non-cryptographic jitter
		jitterFrac := rand.Float64()*jitterFraction - jitterOffset
		delay = time.Duration(float64(delay) * (1.0 + jitterFrac))

		// Cap at maxBackoffDelay.
		if delay > maxBackoffDelay {
			delay = maxBackoffDelay
		}

		c.logger.Debug(ctx, "Waiting before retry", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "retry_backoff",
			logFieldAttempt:   attempt,
			"delay":           delay,
		})
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// parseRateLimitReset extracts reset time from rate limit headers.
func (c *httpClient) parseRateLimitReset(ctx context.Context, resp *http.Response) int64 {
	resetStr := resp.Header.Get("X-Ratelimit-Reset")
	if resetStr == "" {
		resetStr = resp.Header.Get("Retry-After")
	}
	if resetStr == "" {
		return 0
	}

	reset, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		c.logger.Warn(ctx, "Failed to parse rate limit reset header", map[string]interface{}{
			logFieldAdapter:   logAdapterVantage,
			logFieldOperation: "parse_rate_limit",
			logFieldAttempt:   0,
			"value":           resetStr,
			"error":           err,
		})
		return 0
	}

	return reset
}

// redactURL removes sensitive information from URLs for logging.
func (c *httpClient) redactURL(rawURL string) string {
	// Redact Authorization header values in query parameters if present
	if strings.Contains(rawURL, "Authorization=") {
		rawURL = strings.ReplaceAll(rawURL, "Authorization=Bearer%20"+c.token, "Authorization=Bearer%20****")
	}

	// Redact workspace_token and cost_report_token query parameters
	rawURL = redactQueryParam(rawURL, "workspace_token")
	rawURL = redactQueryParam(rawURL, "cost_report_token")

	// Redact any report token segment in cost_reports path.
	rePath := regexp.MustCompile(`/cost_reports/[^/?#]+`)
	rawURL = rePath.ReplaceAllString(rawURL, "/cost_reports/****")

	return rawURL
}

// redactQueryParam redacts a query parameter value from a URL.
func redactQueryParam(rawURL, paramName string) string {
	re := regexp.MustCompile("([?&])" + regexp.QuoteMeta(paramName) + "=([^&]*)")
	return re.ReplaceAllString(rawURL, "$1"+paramName+"=****")
}

// rateLimitError represents a rate limiting error.
type rateLimitError struct {
	resetIn time.Duration
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("rate limited, reset in %v", e.resetIn)
}
