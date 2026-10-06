package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantage/client"
)

func checkWiremockRunning(t *testing.T) {
	// Check if Wiremock is running.
	resp, err := http.Get("http://localhost:8080/__admin/health")
	if err != nil {
		t.Skip("Wiremock server not running. Run 'make wiremock-up' first.")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Skip("Wiremock server not responding correctly")
	}
}

func createTestClient(t *testing.T, baseURL string) client.Client {
	config := client.Config{
		BaseURL:    baseURL,
		Token:      "test-token",
		Timeout:    30 * time.Second,
		MaxRetries: 0, // Disable retries for deterministic testing
		Logger:     client.NewNoopLogger(),
	}

	client, err := client.New(config)
	require.NoError(t, err)
	return client
}

func loadExpectedRecords(t *testing.T, filename string) []CostRecord {
	// Get the directory of the test file.
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	contractsDir := filepath.Join(testDir, "..", "contracts")
	path := filepath.Join(contractsDir, filename)

	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(file)
	require.NoError(t, err)

	var records []CostRecord
	err = json.Unmarshal(data, &records)
	require.NoError(t, err)

	return records
}

func TestContract_CostsMapping(t *testing.T) {
	// Check that Wiremock is running.
	checkWiremockRunning(t)

	// Create client pointing to Wiremock.
	testClient := createTestClient(t, "http://localhost:8080")

	// Create adapter.
	adapter := New(testClient, client.NewNoopLogger())

	// Test query.
	query := client.Query{
		CostReportToken: "cr_test_report",
		StartAt:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:           time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost", "usage"},
	}

	// Fetch data.
	page, err := testClient.Costs(context.Background(), query)
	require.NoError(t, err)

	// Convert to CostRecords.
	var records []CostRecord
	for _, row := range page.Data {
		record := adapter.mapVantageRowToCostRecord(row, query, "test_query_hash", "cost")
		records = append(records, record)
	}

	// Load expected results.
	expectedRecords := loadExpectedRecords(t, "expected_cost_records_page1.json")

	// Compare.
	assert.Len(t, records, len(expectedRecords), "Number of records should match")

	for i, expected := range expectedRecords {
		if i >= len(records) {
			t.Fatalf("Missing record at index %d", i)
		}
		actual := records[i]

		// Compare key fields.
		assert.Equal(t, expected.Timestamp, actual.Timestamp, "Timestamp should match for record %d", i)
		assert.Equal(t, expected.Provider, actual.Provider, "Provider should match for record %d", i)
		assert.Equal(t, expected.Service, actual.Service, "Service should match for record %d", i)
		assert.Equal(t, expected.NetCost, actual.NetCost, "NetCost should match for record %d", i)
		assert.Equal(t, expected.Currency, actual.Currency, "Currency should match for record %d", i)
		assert.Equal(
			t,
			expected.SourceReportToken,
			actual.SourceReportToken,
			"SourceReportToken should match for record %d",
			i,
		)
		assert.Equal(t, expected.QueryHash, actual.QueryHash, "QueryHash should match for record %d", i)
		assert.Equal(t, expected.MetricType, actual.MetricType, "MetricType should match for record %d", i)

		// Compare labels.
		if expected.Labels != nil || actual.Labels != nil {
			assert.Equal(t, expected.Labels, actual.Labels, "Labels should match for record %d", i)
		}
	}
}

func TestContract_ForecastMapping(t *testing.T) {
	// Check that Wiremock is running.
	checkWiremockRunning(t)

	// Create client pointing to Wiremock.
	testClient := createTestClient(t, "http://localhost:8080")

	// Create adapter.
	adapter := New(testClient, client.NewNoopLogger())

	// Test forecast query.
	forecastQuery := client.ForecastQuery{
		StartAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	}

	// Fetch forecast data.
	forecast, err := testClient.Forecast(context.Background(), "cr_test_report", forecastQuery)
	require.NoError(t, err)

	// Convert to CostRecords.
	var records []CostRecord
	for _, row := range forecast.Data {
		// Create a mock CostRow from ForecastRow for mapping.
		costRow := client.CostRow{
			BucketStart: row.BucketStart,
			BucketEnd:   row.BucketEnd,
			Cost:        row.Cost,
			Currency:    row.Currency,
		}
		query := client.Query{
			CostReportToken: "cr_test_report",
			Granularity:     "day",
		}
		record := adapter.mapVantageRowToCostRecord(costRow, query, "test_forecast_hash", "forecast")
		records = append(records, record)
	}

	// Load expected results.
	expectedRecords := loadExpectedRecords(t, "expected_forecast_records.json")

	// Compare.
	assert.Len(t, records, len(expectedRecords), "Number of forecast records should match")

	for i, expected := range expectedRecords {
		if i >= len(records) {
			t.Fatalf("Missing forecast record at index %d", i)
		}
		actual := records[i]

		assert.Equal(t, expected.Timestamp, actual.Timestamp, "Forecast timestamp should match for record %d", i)
		assert.Equal(t, expected.NetCost, actual.NetCost, "Forecast cost should match for record %d", i)
		assert.Equal(t, expected.Currency, actual.Currency, "Forecast currency should match for record %d", i)
		assert.Equal(t, expected.MetricType, actual.MetricType, "Forecast metric type should match for record %d", i)
	}
}

func TestContract_Idempotency(t *testing.T) {
	// Test that same inputs produce same query hashes.
	adapter := New(nil, client.NewNoopLogger())

	query1 := client.Query{
		CostReportToken: "cr_test",
		StartAt:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:           time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
	}

	query2 := client.Query{
		CostReportToken: "cr_test",
		StartAt:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:           time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
	}

	hash1 := adapter.generateQueryHash(query1)
	hash2 := adapter.generateQueryHash(query2)

	assert.Equal(t, hash1, hash2, "Identical queries should produce identical hashes")

	// Different query should produce different hash.
	query3 := query1
	query3.GroupBys = []string{"provider", "region"}
	hash3 := adapter.generateQueryHash(query3)
	assert.NotEqual(t, hash1, hash3, "Different queries should produce different hashes")
}

func TestContract_Pagination(t *testing.T) {
	// Check that Wiremock is running.
	checkWiremockRunning(t)

	// Create client pointing to Wiremock.
	testClient := createTestClient(t, "http://localhost:8080")

	// Test pagination.
	query := client.Query{
		CostReportToken: "cr_test_report",
		StartAt:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:           time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost", "usage"},
	}

	pager := client.NewPager(testClient, query, client.NewNoopLogger())

	// First page.
	page1, err := pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.True(t, page1.HasMore, "First page should indicate more pages available")
	assert.Equal(t, "page2_cursor_abc123", page1.NextCursor, "First page should have correct cursor")
	assert.Len(t, page1.Data, 2, "First page should have 2 records")

	// Second page.
	page2, err := pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.False(t, page2.HasMore, "Second page should indicate no more pages")
	assert.Empty(t, page2.NextCursor, "Second page should have empty cursor")
	assert.Len(t, page2.Data, 1, "Second page should have 1 record")

	// Third page should fail.
	_, err = pager.NextPage(context.Background())
	assert.Error(t, err, "Third page should not exist")
}

// Helper functions.
