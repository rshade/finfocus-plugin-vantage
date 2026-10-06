package adapter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigHappyPath(t *testing.T) {
	// Create a temporary config file.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 0.1
source: vantage

credentials:
  token: test-token-123

params:
  cost_report_token: cr_test123
  start_date: "2024-01-01"
  end_date: "2024-12-31"
  granularity: day
  group_bys:
    - provider
    - service
    - region
  metrics:
    - cost
    - usage
  include_forecast: true
  page_size: 5000
  request_timeout_seconds: 60
  max_retries: 5
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load the config.
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify config values.
	assert.Equal(t, "test-token-123", cfg.Token)
	assert.Equal(t, "cr_test123", cfg.CostReportToken)
	assert.Empty(t, cfg.WorkspaceToken)
	assert.Equal(t, "day", cfg.Granularity)
	assert.Equal(t, 5000, cfg.PageSize)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.True(t, cfg.IncludeForecast)
	assert.Len(t, cfg.GroupBys, 3)
	assert.Len(t, cfg.Metrics, 2)

	// Check dates.
	expectedStart, _ := time.Parse("2006-01-02", "2024-01-01")
	assert.Equal(t, expectedStart, cfg.StartDate)

	expectedEnd, _ := time.Parse("2006-01-02", "2024-12-31")
	require.NotNil(t, cfg.EndDate)
	assert.Equal(t, expectedEnd, *cfg.EndDate)
}

func TestLoadConfigWithEnvironmentOverrides(t *testing.T) {
	// Create a temporary config file.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
credentials:
  token: file-token

params:
  cost_report_token: cr_file
  start_date: "2024-01-01"
  granularity: day
  metrics:
    - cost
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set environment variables.
	t.Setenv("FINFOCUS_VANTAGE_TOKEN", "env-token-override")
	t.Setenv("FINFOCUS_VANTAGE_START_DATE", "2024-02-01")
	t.Setenv("FINFOCUS_VANTAGE_END_DATE", "2024-11-01")

	// Load the config.
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify environment variables override file values.
	assert.Equal(t, "env-token-override", cfg.Token)

	expectedStart, _ := time.Parse("2006-01-02", "2024-02-01")
	assert.Equal(t, expectedStart, cfg.StartDate)

	expectedEnd, _ := time.Parse("2006-01-02", "2024-11-01")
	require.NotNil(t, cfg.EndDate)
	assert.Equal(t, expectedEnd, *cfg.EndDate)
}

func TestLoadConfigDefaultsForOptionalFields(t *testing.T) {
	// Create a minimal config file.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
credentials:
  token: test-token

params:
  cost_report_token: cr_test
  granularity: day
  metrics:
    - cost
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load the config.
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify defaults are applied.
	assert.Equal(t, 5000, cfg.PageSize)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Nil(t, cfg.EndDate)

	// Start date should default to 12 months ago (approximate check).
	now := time.Now()
	expectedApproximateStart := now.AddDate(-1, 0, 0)
	// Allow 1-day tolerance for timing differences.
	assert.True(t, cfg.StartDate.After(expectedApproximateStart.AddDate(0, 0, -1)))
	assert.True(t, cfg.StartDate.Before(expectedApproximateStart.AddDate(0, 0, 1)))
}

// Error case tests.

func TestLoadConfigErrorMissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestLoadConfigErrorEmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config file path cannot be empty")
}

func TestLoadConfigErrorInvalidYAML(t *testing.T) {
	// Create a config file with invalid YAML.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidContent := `
credentials:
  token: test
  invalid: [unclosed array
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestValidateConfigErrorMissingToken(t *testing.T) {
	cfg := &Config{
		Token:           "",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials.token is required")
}

func TestValidateConfigErrorMissingTokenType(t *testing.T) {
	cfg := &Config{
		Token:       "test-token",
		Granularity: "day",
		StartDate:   time.Now(),
		PageSize:    5000,
		Timeout:     60 * time.Second,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "either workspace_token or cost_report_token must be specified")
}

func TestValidateConfigErrorInvalidGranularity(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "invalid",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "granularity must be 'day' or 'month'")
}

func TestValidateConfigErrorEndDateBeforeStartDate(t *testing.T) {
	startDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       startDate,
		EndDate:         &endDate,
		PageSize:        5000,
		Timeout:         60 * time.Second,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end_date must not be before start_date")
}

func TestValidateConfigErrorInvalidPageSize(t *testing.T) {
	tests := []struct {
		name     string
		pageSize int
		errMsg   string
	}{
		{"zero page size", 0, "page_size must be at least 1"},
		{"negative page size", -1, "page_size must be at least 1"},
		{"exceeds max", 15000, "page_size cannot exceed 10000"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Token:           "test-token",
				CostReportToken: "cr_test",
				Granularity:     "day",
				StartDate:       time.Now(),
				PageSize:        test.pageSize,
				Timeout:         60 * time.Second,
			}

			err := ValidateConfig(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.errMsg)
		})
	}
}

func TestValidateConfigErrorInvalidTimeout(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         0,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be at least 1 second")
}

func TestValidateConfigErrorNegativeMaxRetries(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
		MaxRetries:      -1,
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_retries cannot be negative")
}

func TestValidateConfigErrorInvalidGroupBy(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
		GroupBys:        []string{"provider", "invalid_groupby"},
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid group_by value: invalid_groupby")
}

func TestValidateConfigErrorInvalidMetric(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
		Metrics:         []string{"cost", "invalid_metric"},
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metric value: invalid_metric")
}

func TestValidateConfigValidGroupBys(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
		GroupBys:        []string{"provider", "service", "account", "project", "region", "resource_id", "tags"},
	}

	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfigValidMetrics(t *testing.T) {
	cfg := &Config{
		Token:           "test-token",
		CostReportToken: "cr_test",
		Granularity:     "day",
		StartDate:       time.Now(),
		PageSize:        5000,
		Timeout:         60 * time.Second,
		Metrics: []string{
			"cost",
			"usage",
			"effective_unit_price",
			"amortized_cost",
			"taxes",
			"credits",
			"refunds",
		},
	}

	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfigWorkspaceTokenAlternative(t *testing.T) {
	cfg := &Config{
		Token:          "test-token",
		WorkspaceToken: "ws_test",
		Granularity:    "day",
		StartDate:      time.Now(),
		PageSize:       5000,
		Timeout:        60 * time.Second,
	}

	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestLoadConfigInvalidDateFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
credentials:
  token: test-token

params:
  cost_report_token: cr_test
  start_date: "invalid-date"
  granularity: day
  metrics:
    - cost
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid start_date format")
}

func TestLoadConfigInvalidEndDateFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
credentials:
  token: test-token

params:
  cost_report_token: cr_test
  start_date: "2024-01-01"
  end_date: "not-a-date"
  granularity: day
  metrics:
    - cost
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid end_date format")
}
