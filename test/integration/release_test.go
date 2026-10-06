package integration_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFirstReleaseConfiguration(t *testing.T) {
	configBytes, err := os.ReadFile("../../release-please-config.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Packages map[string]struct {
			IncludeComponentInTag *bool  `json:"include-component-in-tag"`
			InitialVersion        string `json:"initial-version"`
		} `json:"packages"`
	}
	if err = json.Unmarshal(configBytes, &config); err != nil {
		t.Fatal(err)
	}
	root, ok := config.Packages["."]
	if !ok || root.IncludeComponentInTag == nil || *root.IncludeComponentInTag {
		t.Fatal("root package must explicitly disable component-prefixed tags")
	}
	if root.InitialVersion != "0.1.0" {
		t.Fatal("first release must be v0.1.0")
	}
	manifestBytes, err := os.ReadFile("../../.release-please-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["."] == "" {
		t.Fatal("root release version is missing")
	}
	// v0.1.0 is published, so the manifest records it and the next release is v0.1.1.
	if manifest["."] != "0.1.0" {
		t.Fatal("manifest must record the published v0.1.0")
	}
}

func TestReleaseArchiveNaming(t *testing.T) {
	config, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Every plugin in the family names archives finfocus-plugin-<name>_<version>_<Os>_<arch>.
	if !strings.Contains(string(config), "name_template: 'finfocus-plugin-vantage_{{ .Version }}_") {
		t.Fatal("archives must use the finfocus-plugin-vantage prefix")
	}
	if !strings.Contains(string(config), "name_template: 'checksums.txt'") {
		t.Fatal("installer requires checksums.txt")
	}
}

func TestReleaseWorkflowRunsOnReleaseCreated(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(workflow)
	// release-please creates the tag through the API, so a push-tag trigger never fires.
	for _, want := range []string{
		"types: [created]",
		"workflow_dispatch:",
		"ref: ${{ inputs.tag || github.event.release.tag_name }}",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release.yml must contain %q", want)
		}
	}
	if strings.Contains(text, "tags:") {
		t.Fatal("release.yml must not trigger on pushed tags")
	}
}
