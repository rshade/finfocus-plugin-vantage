package adapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantage/client"
)

// mockSink implements the Sink interface for testing.
type mockSink struct {
	mock.Mock

	records   []CostRecord
	bookmarks map[string]string
}

func (m *mockSink) WriteRecords(ctx context.Context, records []CostRecord) error {
	args := m.Called(ctx, records)
	m.records = append(m.records, records...)
	return args.Error(0)
}

func (m *mockSink) GetBookmark(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *mockSink) SetBookmark(ctx context.Context, key string, value string) error {
	args := m.Called(ctx, key, value)
	if m.bookmarks == nil {
		m.bookmarks = make(map[string]string)
	}
	m.bookmarks[key] = value
	return args.Error(0)
}

// mockClient implements the client.Client interface for testing.
type mockClient struct {
	mock.Mock
}

func (m *mockClient) Costs(ctx context.Context, query client.Query) (client.Page, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(client.Page), args.Error(1)
}

func (m *mockClient) Forecast(
	ctx context.Context,
	reportToken string,
	query client.ForecastQuery,
) (client.Forecast, error) {
	args := m.Called(ctx, reportToken, query)
	return args.Get(0).(client.Forecast), args.Error(1)
}

func TestAdapter_mapVantageRowToCostRecord(t *testing.T) {
	logger := client.NewNoopLogger()
	adapter := New(&mockClient{}, logger)

	row := client.CostRow{
		Provider:   "aws",
		Service:    "EC2",
		Account:    "123456789",
		Project:    "my-project",
		Region:     "us-east-1",
		ResourceID: "i-1234567890abcdef0",
		Tags: map[string]string{
			"Environment":      "production",
			"Team":             "backend",
			"user:cost-center": "engineering",
		},
		Cost:          100.50,
		UsageQuantity: 720.0,
		UsageUnit:     "hours",
		ListCost:      120.00,
		AmortizedCost: 95.00,
		Tax:           8.50,
		Credit:        5.00,
		Refund:        0.0,
		Currency:      "USD",
		BucketStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		BucketEnd:     time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	query := client.Query{
		CostReportToken: "cr_test",
		Granularity:     "day",
	}

	record := adapter.mapVantageRowToCostRecord(row, query, "test-hash", "cost")

	assert.Equal(t, row.BucketStart, record.Timestamp)
	assert.Equal(t, "aws", record.Provider)
	assert.Equal(t, "EC2", record.Service)
	assert.Equal(t, "123456789", record.AccountID)
	assert.Equal(t, "my-project", record.Project)
	assert.Equal(t, "us-east-1", record.Region)
	assert.Equal(t, "i-1234567890abcdef0", record.ResourceID)
	assert.Equal(t, "USD", record.Currency)
	assert.Equal(t, "cr_test", record.SourceReportToken)
	assert.Equal(t, "test-hash", record.QueryHash)
	assert.Equal(t, "cost", record.MetricType)

	// Check cost values.
	assert.NotNil(t, record.NetCost)
	assert.InEpsilon(t, 100.50, *record.NetCost, 0.01)
	assert.NotNil(t, record.ListCost)
	assert.InEpsilon(t, 120.00, *record.ListCost, 0.01)
	assert.NotNil(t, record.AmortizedCost)
	assert.InEpsilon(t, 95.00, *record.AmortizedCost, 0.01)
	assert.NotNil(t, record.TaxCost)
	assert.InEpsilon(t, 8.50, *record.TaxCost, 0.01)
	assert.NotNil(t, record.CreditAmount)
	assert.InEpsilon(t, 5.00, *record.CreditAmount, 0.01)

	// Check usage.
	assert.NotNil(t, record.UsageAmount)
	assert.InEpsilon(t, 720.0, *record.UsageAmount, 0.01)
	assert.Equal(t, "hours", record.UsageUnit)

	// Check normalized tags.
	expectedLabels := map[string]string{
		"environment":      "production",
		"team":             "backend",
		"user:cost-center": "engineering",
	}
	assert.Equal(t, expectedLabels, record.Labels)

	// Check diagnostics (should be nil for complete data).
	assert.Nil(t, record.Diagnostics)
}

func TestAdapter_mapVantageRowToCostRecord_WithMissingFields(t *testing.T) {
	logger := client.NewNoopLogger()
	adapter := New(&mockClient{}, logger)

	row := client.CostRow{
		// Missing provider and service.
		Cost:        0, // Zero cost
		BucketStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		BucketEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	query := client.Query{
		CostReportToken: "cr_test",
	}

	record := adapter.mapVantageRowToCostRecord(row, query, "test-hash", "cost")

	// Check that diagnostics are present.
	assert.NotNil(t, record.Diagnostics)
	assert.Contains(t, record.Diagnostics.MissingFields, "provider")
	assert.Contains(t, record.Diagnostics.MissingFields, "service")
	assert.Contains(t, record.Diagnostics.MissingFields, "net_cost")
	assert.Equal(t, "required FOCUS 1.2 field cloud_provider is empty", record.Diagnostics.MissingFields["provider"])
	assert.Equal(t, "required FOCUS 1.2 field service_name is empty", record.Diagnostics.MissingFields["service"])
	assert.Equal(t, "required FOCUS 1.2 field net_cost is nil or zero", record.Diagnostics.MissingFields["net_cost"])
}

func TestAdapter_generateQueryHash(t *testing.T) {
	logger := client.NewNoopLogger()
	adapter := New(&mockClient{}, logger)

	query := client.Query{
		WorkspaceToken:  "ws_test",
		CostReportToken: "cr_test",
		StartAt:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:           time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		Granularity:     "day",
		GroupBys:        []string{"provider", "service", "account"},
		Metrics:         []string{"cost", "usage"},
	}

	hash1 := adapter.generateQueryHash(query)
	hash2 := adapter.generateQueryHash(query)

	// Hash should be deterministic.
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)

	// Different query should produce different hash.
	query2 := query
	query2.GroupBys = []string{"provider", "region"}
	hash3 := adapter.generateQueryHash(query2)
	assert.NotEqual(t, hash1, hash3)
}

func TestNormalizeTagKey(t *testing.T) {
	logger := client.NewNoopLogger()
	adapter := New(&mockClient{}, logger)

	tests := []struct {
		input    string
		expected string
	}{
		{"Environment", "environment"},
		{"Cost_Center", "cost-center"},
		{"kubernetes.io/cluster", "kubernetes.io/cluster"},
		{"User:Team", "user:team"},
		{"Multiple___Underscores", "multiple-underscores"},
		{"--leading-trailing--", "leading-trailing"},
		{"NormalKey", "normalkey"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.normalizeTagKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	logger := client.NewNoopLogger()
	adapter := New(&mockClient{}, logger)

	input := map[string]string{
		"Environment":           "production",
		"Cost_Center":           "engineering",
		"user:team":             "backend",
		"kubernetes.io/pod-uid": "12345", // Should be filtered out
		"NormalTag":             "value",
	}

	result := adapter.normalizeTags(input)

	expected := map[string]string{
		"environment": "production",
		"cost-center": "engineering",
		"user:team":   "backend",
		"normaltag":   "value",
	}

	assert.Equal(t, expected, result)
}

func TestAdapter_SyncIncremental(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	// Mock empty response.
	mockClient.On("Costs", mock.Anything, mock.AnythingOfType("client.Query")).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	// Mock sink operations.
	mockSink.On("GetBookmark", mock.Anything, mock.Anything).Return("", nil)
	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)
	mockSink.On("SetBookmark", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := adapter.Sync(context.Background(), cfg, mockSink)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncBackfill(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		CostReportToken: "cr_test",
		StartDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         &endDate,
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	// Mock empty response.
	mockClient.On("Costs", mock.Anything, mock.AnythingOfType("client.Query")).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	// Mock sink operations.
	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)

	err := adapter.Sync(context.Background(), cfg, mockSink)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncChunked(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	// Mock responses for each month chunk.
	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.StartAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) &&
			q.EndAt.Equal(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC))
	})).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)

	err := adapter.syncChunked(context.Background(), cfg, mockSink, startDate, endDate)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncChunked_MultipleChunks(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	// Mock responses for January and February chunks.
	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.StartAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) &&
			q.EndAt.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	})).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.StartAt.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)) &&
			q.EndAt.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	})).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)

	err := adapter.syncChunked(context.Background(), cfg, mockSink, startDate, endDate)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncForecast(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	startDate := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	forecastData := []client.ForecastRow{
		{
			BucketStart: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			BucketEnd:   time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			Cost:        100.50,
			Currency:    "USD",
		},
	}

	mockClient.On("Forecast", mock.Anything, "cr_test", mock.AnythingOfType("client.ForecastQuery")).
		Return(client.Forecast{
			Data: forecastData,
		}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.MatchedBy(func(records []CostRecord) bool {
		return len(records) == 1 && *records[0].NetCost == 100.50
	})).Return(nil)

	err := adapter.syncForecast(context.Background(), cfg, mockSink, startDate, endDate, "query_hash")

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncForecast_Error(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
	}

	startDate := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	mockClient.On("Forecast", mock.Anything, "cr_test", mock.AnythingOfType("client.ForecastQuery")).
		Return(client.Forecast{}, errors.New("forecast error"))

	err := adapter.syncForecast(context.Background(), cfg, mockSink, startDate, endDate, "query_hash")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching forecast")
	mockClient.AssertExpectations(t)
}

func TestAdapter_SyncSingleRange_WithData(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	costData := []client.CostRow{
		{
			BucketStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			BucketEnd:     time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Provider:      "aws",
			Service:       "ec2",
			Cost:          50.25,
			Currency:      "USD",
			UsageUnit:     "Hrs",
			UsageQuantity: 24.0,
		},
	}

	mockClient.On("Costs", mock.Anything, mock.AnythingOfType("client.Query")).Return(client.Page{
		Data:       costData,
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.MatchedBy(func(records []CostRecord) bool {
		return len(records) == 1 && *records[0].NetCost == 50.25
	})).Return(nil)

	err := adapter.syncSingleRange(context.Background(), cfg, mockSink, startDate, endDate, true)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncSingleRange_Pagination(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        1, // Force pagination
	}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	costData1 := []client.CostRow{
		{
			BucketStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			BucketEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Provider:    "aws",
			Service:     "ec2",
			Cost:        50.25,
			Currency:    "USD",
		},
	}

	costData2 := []client.CostRow{
		{
			BucketStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			BucketEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Provider:    "aws",
			Service:     "s3",
			Cost:        25.75,
			Currency:    "USD",
		},
	}

	// Mock first page.
	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.Cursor == ""
	})).Return(client.Page{
		Data:       costData1,
		NextCursor: "cursor1",
		HasMore:    true,
	}, nil)

	// Mock second page.
	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.Cursor == "cursor1"
	})).Return(client.Page{
		Data:       costData2,
		NextCursor: "",
		HasMore:    false,
	}, nil)

	// Expect one call to WriteRecords with all records combined.
	mockSink.On("WriteRecords", mock.Anything, mock.MatchedBy(func(records []CostRecord) bool {
		return len(records) == 2 && *records[0].NetCost == 50.25 && *records[1].NetCost == 25.75
	})).Return(nil)

	err := adapter.syncSingleRange(context.Background(), cfg, mockSink, startDate, endDate, true)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncSingleRange_Error(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
	}

	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	mockClient.On("Costs", mock.Anything, mock.AnythingOfType("client.Query")).
		Return(client.Page{}, errors.New("costs error"))

	err := adapter.syncSingleRange(context.Background(), cfg, mockSink, startDate, endDate, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching costs")
	mockClient.AssertExpectations(t)
}

func TestAdapter_SyncDateRange_BackfillChunking(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	// Date range > 30 days to trigger chunking.
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC) // 60 days

	// Mock responses for January and February chunks.
	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.StartAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) &&
			q.EndAt.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	})).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockClient.On("Costs", mock.Anything, mock.MatchedBy(func(q client.Query) bool {
		return q.StartAt.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)) &&
			q.EndAt.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	})).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)

	err := adapter.syncDateRange(context.Background(), cfg, mockSink, startDate, endDate, true)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestAdapter_SyncDateRange_SingleRange(t *testing.T) {
	mockClient := &mockClient{}
	mockSink := &mockSink{}

	logger := client.NewNoopLogger()
	adapter := New(mockClient, logger)

	cfg := Config{
		CostReportToken: "cr_test",
		Granularity:     "day",
		GroupBys:        []string{"provider", "service"},
		Metrics:         []string{"cost"},
		PageSize:        100,
	}

	// Date range <= 30 days, should use single range.
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	mockClient.On("Costs", mock.Anything, mock.AnythingOfType("client.Query")).Return(client.Page{
		Data:       []client.CostRow{},
		NextCursor: "",
		HasMore:    false,
	}, nil)

	mockSink.On("WriteRecords", mock.Anything, mock.Anything).Return(nil)

	err := adapter.syncDateRange(context.Background(), cfg, mockSink, startDate, endDate, true)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockSink.AssertExpectations(t)
}

func TestDiagnostics(t *testing.T) {
	diag := NewDiagnostics()

	// Test adding missing fields.
	diag.AddMissingField("provider", "test reason 1")
	diag.AddMissingField("service", "test reason 2")

	// Test adding warnings.
	diag.AddWarning("negative_cost")
	diag.AddWarning("missing_unit")

	// Test source info - this should trigger the nil initialization branch.
	diag.SetSourceInfo("api_version", "v1")
	diag.SetSourceInfo("record_count", 100)

	assert.True(t, diag.HasIssues())
	assert.Len(t, diag.MissingFields, 2)
	assert.Len(t, diag.Warnings, 2)
	assert.Equal(t, "v1", diag.SourceInfo["api_version"])
	assert.Equal(t, 100, diag.SourceInfo["record_count"])

	// Test SetSourceInfo on nil map (separate diagnostics instance).
	diag2 := &Diagnostics{}
	diag2.SetSourceInfo("test_key", "test_value")
	assert.Equal(t, "test_value", diag2.SourceInfo["test_key"])
}
