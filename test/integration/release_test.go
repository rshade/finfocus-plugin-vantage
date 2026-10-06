package integration_test

import (
	"encoding/json"
	"os"
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
	// Until the first release is published, the manifest must not claim it shipped.
	if manifest["."] != "0.0.0" {
		t.Fatal("unexpected first-release manifest version")
	}
}
