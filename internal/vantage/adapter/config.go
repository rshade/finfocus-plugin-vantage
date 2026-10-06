package adapter

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

const (
	defaultTimeoutSeconds = 60
	defaultPageSize       = 5000
	maxPageSize           = 10000
	defaultMaxRetries     = 5
)

// Config holds the configuration for the Vantage adapter.
type Config struct {
	Token           string        `yaml:"token"                       json:"token"`
	WorkspaceToken  string        `yaml:"workspace_token,omitempty"   json:"workspace_token,omitempty"`
	CostReportToken string        `yaml:"cost_report_token,omitempty" json:"cost_report_token,omitempty"`
	StartDate       time.Time     `yaml:"start_date"                  json:"start_date"`
	EndDate         *time.Time    `yaml:"end_date,omitempty"          json:"end_date,omitempty"`
	Granularity     string        `yaml:"granularity"                 json:"granularity"`
	GroupBys        []string      `yaml:"group_bys"                   json:"group_bys"`
	Metrics         []string      `yaml:"metrics"                     json:"metrics"`
	IncludeForecast bool          `yaml:"include_forecast"            json:"include_forecast"`
	PageSize        int           `yaml:"page_size"                   json:"page_size"`
	Timeout         time.Duration `yaml:"timeout"                     json:"timeout"`
	MaxRetries      int           `yaml:"max_retries"                 json:"max_retries"`
}

// rawConfig is an intermediate struct for unmarshaling YAML with flexible types.
type rawConfig struct {
	Credentials map[string]interface{} `yaml:"credentials"`
	Params      map[string]interface{} `yaml:"params"`
}

// parseCredentials extracts token from raw config and applies env overrides.
func parseCredentials(raw *rawConfig) string {
	var token string
	if raw.Credentials != nil {
		if t, ok := raw.Credentials["token"].(string); ok {
			token = t
		}
	}
	if envToken := os.Getenv("FINFOCUS_VANTAGE_TOKEN"); envToken != "" {
		token = envToken
	}
	return token
}

// parseParams extracts params from raw config.
func parseParams(raw *rawConfig) (string, string, string, string, string, []string, []string, bool, int, int, int) {
	var workspaceToken, costReportToken, granularityStr, startDateStr, endDateStr string
	var groupBys, metrics []string
	var includeForecast bool
	var pageSize, requestTimeoutSeconds, maxRetries int

	if raw.Params == nil {
		return workspaceToken, costReportToken, granularityStr, startDateStr, endDateStr, groupBys, metrics, includeForecast, pageSize, requestTimeoutSeconds, maxRetries
	}

	workspaceToken = cast.ToString(raw.Params["workspace_token"])
	costReportToken = cast.ToString(raw.Params["cost_report_token"])
	granularityStr = cast.ToString(raw.Params["granularity"])
	startDateStr = cast.ToString(raw.Params["start_date"])
	endDateStr = cast.ToString(raw.Params["end_date"])
	groupBys = cast.ToStringSlice(raw.Params["group_bys"])
	metrics = cast.ToStringSlice(raw.Params["metrics"])
	includeForecast = cast.ToBool(raw.Params["include_forecast"])
	pageSize = cast.ToInt(raw.Params["page_size"])
	requestTimeoutSeconds = cast.ToInt(raw.Params["request_timeout_seconds"])
	maxRetries = cast.ToInt(raw.Params["max_retries"])

	return workspaceToken, costReportToken, granularityStr, startDateStr, endDateStr, groupBys, metrics, includeForecast, pageSize, requestTimeoutSeconds, maxRetries
}

// parseDates parses start and end dates with env overrides.
func parseDates(startDateStr, endDateStr string) (time.Time, *time.Time, error) {
	var startDate time.Time
	if envStartDate := os.Getenv("FINFOCUS_VANTAGE_START_DATE"); envStartDate != "" {
		startDateStr = envStartDate
	}
	if startDateStr == "" {
		startDate = time.Now().UTC().AddDate(-1, 0, 0)
	} else {
		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return time.Time{}, nil, fmt.Errorf("invalid start_date format (expected YYYY-MM-DD): %s", startDateStr)
		}
	}

	var endDate *time.Time
	if envEndDate := os.Getenv("FINFOCUS_VANTAGE_END_DATE"); envEndDate != "" {
		endDateStr = envEndDate
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return time.Time{}, nil, fmt.Errorf("invalid end_date format (expected YYYY-MM-DD): %s", endDateStr)
		}
		endDate = &parsed
	}
	return startDate, endDate, nil
}

// LoadConfig loads and parses the config from a YAML file, applying environment variable overrides.
func LoadConfig(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, errors.New("config file path cannot be empty")
	}

	// Check if file exists.
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("config file not found: %s", filePath)
	}

	// Parse YAML file.
	v := viper.New()
	v.SetConfigFile(filePath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into intermediate struct.
	var raw rawConfig
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	token := parseCredentials(&raw)
	workspaceToken, costReportToken, granularityStr, startDateStr, endDateStr, groupBys, metrics, includeForecast, pageSize, requestTimeoutSeconds, maxRetries := parseParams(
		&raw,
	)

	startDate, endDate, err := parseDates(startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	// Build Config struct.
	cfg := &Config{
		Token:           token,
		WorkspaceToken:  workspaceToken,
		CostReportToken: costReportToken,
		StartDate:       startDate,
		EndDate:         endDate,
		Granularity:     granularityStr,
		GroupBys:        groupBys,
		Metrics:         metrics,
		IncludeForecast: includeForecast,
		PageSize:        pageSize,
		MaxRetries:      maxRetries,
	}

	// Set timeout (convert seconds to duration).
	if requestTimeoutSeconds > 0 {
		cfg.Timeout = time.Duration(requestTimeoutSeconds) * time.Second
	} else {
		cfg.Timeout = defaultTimeoutSeconds * time.Second
	}

	// Set page size default.
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}

	// Set max retries default.
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}

	// Validate the config.
	if validErr := ValidateConfig(cfg); validErr != nil {
		return nil, validErr
	}

	return cfg, nil
}

// ValidateConfig validates all configuration fields and returns clear error messages.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	// Token validation.
	if cfg.Token == "" {
		return errors.New(
			"credentials.token is required (set via YAML or FINFOCUS_VANTAGE_TOKEN environment variable)",
		)
	}

	// At least one token type must be provided.
	if cfg.WorkspaceToken == "" && cfg.CostReportToken == "" {
		return errors.New("either workspace_token or cost_report_token must be specified in params")
	}

	// Granularity validation.
	if cfg.Granularity == "" {
		return errors.New("granularity must be specified in params")
	}
	if cfg.Granularity != "day" && cfg.Granularity != "month" {
		return fmt.Errorf("granularity must be 'day' or 'month', got: %s", cfg.Granularity)
	}

	// Start date validation.
	if cfg.StartDate.IsZero() {
		return errors.New("start_date must be a valid ISO date (YYYY-MM-DD)")
	}

	// End date vs start date validation.
	if cfg.EndDate != nil && cfg.EndDate.Before(cfg.StartDate) {
		return errors.New("end_date must not be before start_date")
	}

	// Page size validation.
	if cfg.PageSize < 1 {
		return errors.New("page_size must be at least 1")
	}
	if cfg.PageSize > maxPageSize {
		return fmt.Errorf("page_size cannot exceed %d", maxPageSize)
	}

	// Timeout validation.
	if cfg.Timeout < 1*time.Second {
		return errors.New("timeout must be at least 1 second")
	}

	// Max retries validation.
	if cfg.MaxRetries < 0 {
		return errors.New("max_retries cannot be negative")
	}

	// Group bys validation (should not be empty if specified).
	// Empty list is allowed (will use defaults), but if present should have valid values.
	validGroupBys := map[string]bool{
		dimProvider:   true,
		dimService:    true,
		"account":     true,
		"project":     true,
		"region":      true,
		"resource_id": true,
		"tags":        true,
	}
	for _, gb := range cfg.GroupBys {
		if !validGroupBys[gb] {
			return fmt.Errorf(
				"invalid group_by value: %s (valid: provider, service, account, project, region, resource_id, tags)",
				gb,
			)
		}
	}

	// Metrics validation.
	validMetrics := map[string]bool{
		"cost":                 true,
		"usage":                true,
		"effective_unit_price": true,
		"amortized_cost":       true,
		"taxes":                true,
		"credits":              true,
		"refunds":              true,
	}
	for _, m := range cfg.Metrics {
		if !validMetrics[m] {
			return fmt.Errorf(
				"invalid metric value: %s (valid: cost, usage, effective_unit_price, amortized_cost, taxes, credits, refunds)",
				m,
			)
		}
	}

	return nil
}
