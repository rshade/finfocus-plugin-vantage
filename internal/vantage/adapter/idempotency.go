package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/rshade/finfocus-plugin-vantage/internal/vantage/client"
)

// GenerateLineItemID creates a deterministic idempotency key for a cost record.
// The key is based on the hash of (report_token, date, dimensions, metrics).
// This ensures that identical cost records always produce the same ID, enabling.
// deduplication and idempotent writes to the data store.
func GenerateLineItemID(
	reportToken string,
	row client.CostRow,
	metrics []string,
) string {
	// Create a stable string representation with all relevant fields.
	parts := []string{
		reportToken,
		row.BucketStart.Format("2006-01-02"), // Date only, not time
	}

	// Add dimensions in fixed order for consistency.
	parts = append(parts, row.Provider)
	parts = append(parts, row.Service)
	parts = append(parts, row.Account)
	parts = append(parts, row.Project)
	parts = append(parts, row.Region)
	parts = append(parts, row.ResourceID)

	// Add tags in sorted order by key.
	if len(row.Tags) > 0 {
		tagParts := make([]string, 0, len(row.Tags))
		for k, v := range row.Tags {
			tagParts = append(tagParts, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(tagParts)
		parts = append(parts, strings.Join(tagParts, ";"))
	} else {
		parts = append(parts, "")
	}

	// Add metrics in sorted order by value for consistency.
	sortedMetrics := make([]string, len(metrics))
	copy(sortedMetrics, metrics)
	sort.Strings(sortedMetrics)
	parts = append(parts, strings.Join(sortedMetrics, ","))

	// Add metric values in a consistent order.
	parts = append(parts, fmt.Sprintf("%.16g", row.Cost))
	parts = append(parts, fmt.Sprintf("%.16g", row.UsageQuantity))
	parts = append(parts, fmt.Sprintf("%.16g", row.EffectiveUnitPrice))
	parts = append(parts, fmt.Sprintf("%.16g", row.ListCost))
	parts = append(parts, fmt.Sprintf("%.16g", row.AmortizedCost))
	parts = append(parts, fmt.Sprintf("%.16g", row.Tax))
	parts = append(parts, fmt.Sprintf("%.16g", row.Credit))
	parts = append(parts, fmt.Sprintf("%.16g", row.Refund))
	parts = append(parts, row.UsageUnit)
	parts = append(parts, row.Currency)

	// Generate hash.
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:16]) // First 32 hex chars (128 bits)
}
