package integration_test

import (
	"encoding/json"
	"os"
	"strconv"
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
			BumpPatchPreMajor     *bool  `json:"bump-patch-for-minor-pre-major"`
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
	if root.BumpPatchPreMajor == nil || !*root.BumpPatchPreMajor {
		t.Fatal("bump-patch-for-minor-pre-major must be true to match the family release config")
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
	// Release Please bumps the manifest on every release PR, so assert the
	// published v0.1.0 floor rather than pinning one version.
	parts := strings.Split(manifest["."], ".")
	if len(parts) != 3 {
		t.Fatalf("manifest version %q must be MAJOR.MINOR.PATCH", manifest["."])
	}
	var nums [3]int
	for i, part := range parts {
		if nums[i], err = strconv.Atoi(part); err != nil || nums[i] < 0 {
			t.Fatalf("manifest version %q must be numeric MAJOR.MINOR.PATCH", manifest["."])
		}
	}
	if nums[0] == 0 && nums[1] == 0 {
		t.Fatalf("manifest version %q must not precede the published v0.1.0", manifest["."])
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

func TestReleaseOnlyPublishesArchives(t *testing.T) {
	workflows, err := os.ReadDir("../../.github/workflows")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range workflows {
		body, readErr := os.ReadFile("../../.github/workflows/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(body), "docker/build-push-action") {
			t.Fatalf("%s builds a container image; releases publish archives and checksums only", entry.Name())
		}
	}
	goreleaser, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"dockers", "dockers_v2", "brews", "homebrew_casks", "nfpms"} {
		if strings.Contains(string(goreleaser), "\n"+section+":") {
			t.Fatalf(".goreleaser.yaml must not define %s", section)
		}
	}
}
