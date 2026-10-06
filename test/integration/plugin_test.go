package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPluginBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM lifecycle test requires Unix")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	binary := buildPlugin(ctx, t)
	t.Run("version and graceful RPC lifecycle", func(t *testing.T) { testBinaryLifecycle(ctx, t, binary) })
	t.Run("FinFocus actual cost against mock API", func(t *testing.T) { testCoreActualCost(ctx, t, binary) })
}

func buildPlugin(ctx context.Context, t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "finfocus-plugin-vantage")
	build := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-ldflags",
		"-X main.version=v0.1.0-test",
		"-o",
		binary,
		"../../cmd/finfocus-plugin-vantage",
	)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

func testBinaryLifecycle(ctx context.Context, t *testing.T, binary string) {
	t.Helper()

	out, err := exec.CommandContext(ctx, binary, "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "finfocus-plugin-vantage v0.1.0-test" {
		t.Fatalf("version: %s, %v", out, err)
	}
	command := exec.CommandContext(ctx, binary)
	command.Env = append(os.Environ(), "FINFOCUS_PLUGIN_PORT=0")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			_ = command.Process.Kill()
			<-done
		}
	}()
	portLine := make(chan string, 1)
	go func() { reader := bufio.NewReader(stdout); line, _ := reader.ReadString('\n'); portLine <- line }()
	var line string
	select {
	case line = <-portLine:
	case <-time.After(2 * time.Second):
		t.Fatal("PORT announcement took longer than two seconds")
	}
	var port int
	if _, err = fmt.Sscanf(line, "PORT=%d", &port); err != nil || port <= 0 {
		t.Fatalf("startup: %q, %v", line, err)
	}
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	client := pbc.NewCostSourceServiceClient(conn)
	rpcCtx, rpcCancel := context.WithTimeout(ctx, 3*time.Second)
	defer rpcCancel()
	name, err := client.Name(rpcCtx, &pbc.NameRequest{})
	if err != nil || name.GetName() != "vantage" {
		t.Fatalf("Name=%v, %v", name, err)
	}
	info, err := client.GetPluginInfo(rpcCtx, &pbc.GetPluginInfoRequest{})
	if err != nil || info.GetVersion() != "v0.1.0-test" ||
		info.GetMetadata()["supports_per_request_credentials"] != "true" {
		t.Fatalf("metadata=%v, %v", info, err)
	}
	if err = command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		stopped = true
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("plugin did not shut down gracefully")
	}
}

func testCoreActualCost(ctx context.Context, t *testing.T, binary string) {
	t.Helper()

	core := os.Getenv("FINFOCUS_CORE_BINARY")
	if core == "" {
		t.Skip("set FINFOCUS_CORE_BINARY to run the optional core acceptance test")
	}
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "vantage", "v0.1.0-test")
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(pluginDir, "finfocus-plugin-vantage"), data, 0700); err != nil {
		t.Fatal(err)
	}
	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		if r.URL.Path != "/v2/costs" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			t.Error("incorrect API authentication")
		}
		wantFilter := "costs.provider = 'aws' AND costs.service = 'Amazon Elastic Compute Cloud - Compute' AND " +
			"costs.resource_id = 'i-123' AND costs.region = 'us-east-1'"
		if got := r.URL.Query().Get("filter"); got != wantFilter {
			t.Errorf("core filter=%q, want %q", got, wantFilter)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"costs":[{"amount":"6.00","currency":"USD","accrued_at":"2026-01-02"}],"total_cost":{"amount":"6.00","currency":"USD"}}`,
			),
		)
	}))
	defer server.Close()
	plan := filepath.Join(root, "state.json")
	data = []byte(
		`{"version":3,"deployment":{"resources":[{"urn":"urn:pulumi:dev::demo::aws:ec2/instance:Instance::vm","type":"aws:ec2/instance:Instance","id":"i-123","custom":true,"created":"2026-01-01T00:00:00Z","inputs":{"instanceType":"t3.micro","region":"us-east-1"}}]}}`,
	)
	if err = os.WriteFile(plan, data, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(
		ctx,
		core,
		"cost",
		"actual",
		"--pulumi-state",
		plan,
		"--from",
		"2026-01-02",
		"--to",
		"2026-01-03",
		"--adapter",
		"vantage",
		"--output",
		"json",
	)
	command.Dir = root
	command.Env = append(
		os.Environ(),
		"FINFOCUS_HOME="+root,
		"FINFOCUS_PLUGIN_DIR="+filepath.Join(root, "plugins"),
		"FINFOCUS_VANTAGE_TOKEN=mock-token",
		"FINFOCUS_VANTAGE_COST_REPORT_TOKEN=report-token",
		"FINFOCUS_VANTAGE_BASE_URL="+server.URL+"/v2",
	)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("core query: %v\n%s", err, out)
	}
	var results []struct {
		TotalCost float64 `json:"totalCost"`
		Currency  string  `json:"currency"`
		Adapter   string  `json:"adapter"`
	}
	if err = json.Unmarshal(out, &results); err != nil {
		t.Fatalf("core JSON: %v\n%s", err, out)
	}
	if len(results) != 1 || results[0].TotalCost != 6 || results[0].Currency != "USD" || apiCalls.Load() != 1 {
		t.Fatalf("core did not return mock API costs: %s (calls=%d)", out, apiCalls.Load())
	}
}
