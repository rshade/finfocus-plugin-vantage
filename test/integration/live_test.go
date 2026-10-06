package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-vantage/internal/plugin"
)

func TestLiveVantageActualCost(t *testing.T) {
	if os.Getenv("FINFOCUS_VANTAGE_LIVE_TEST") != "1" {
		t.Skip("live-account validation requires FINFOCUS_VANTAGE_LIVE_TEST=1")
	}
	for _, key := range []string{"FINFOCUS_VANTAGE_TOKEN", "FINFOCUS_VANTAGE_COST_REPORT_TOKEN", "FINFOCUS_VANTAGE_TEST_RESOURCE", "FINFOCUS_VANTAGE_TEST_PROVIDER", "FINFOCUS_VANTAGE_TEST_SERVICE", "FINFOCUS_VANTAGE_TEST_START", "FINFOCUS_VANTAGE_TEST_END"} {
		if os.Getenv(key) == "" {
			t.Fatalf("%s is required", key)
		}
	}
	start, err := time.Parse("2006-01-02", os.Getenv("FINFOCUS_VANTAGE_TEST_START"))
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse("2006-01-02", os.Getenv("FINFOCUS_VANTAGE_TEST_END"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	response, err := plugin.New("test").GetActualCost(ctx, &pbc.GetActualCostRequest{
		ResourceId: os.Getenv(
			"FINFOCUS_VANTAGE_TEST_RESOURCE",
		),
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
		Tags: map[string]string{
			"provider": os.Getenv("FINFOCUS_VANTAGE_TEST_PROVIDER"),
			"service":  os.Getenv("FINFOCUS_VANTAGE_TEST_SERVICE"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.GetResults()) == 0 {
		t.Fatal("live validation requires an imported resource with costs in the test window")
	}
}
