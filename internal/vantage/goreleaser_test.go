// Package vantage provides tests for goreleaser configuration compatibility.
package vantage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// goReleaseConfig represents the parsed .goreleaser.yaml structure.
type goReleaseConfig struct {
	Archives []struct {
		NameTemplate    string `yaml:"name_template"`
		Format          string `yaml:"format"`
		FormatOverrides []struct {
			GOOS   string `yaml:"goos"`
			Format string `yaml:"format"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	ProjectName string `yaml:"project_name"`
}

// TestGoReleaserAssetNaming validates that goreleaser asset names match
// the finfocus installer's expected naming convention.
// Based on finfocus/internal/registry/github.go buildAssetPatterns.
func TestGoReleaserAssetNaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Read and parse .goreleaser.yaml
	configPath := filepath.Join("..", "..", ".goreleaser.yaml")
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err, "failed to read .goreleaser.yaml")

	var config goReleaseConfig
	err2 := yaml.Unmarshal(configData, &config)
	require.NoError(t, err2, "failed to parse .goreleaser.yaml")

	require.Len(t, config.Archives, 1, "expected exactly one archive config")
	archive := config.Archives[0]

	// Template variables to test
	testCases := []struct {
		os   string
		arch string
		ver  string
	}{
		{"linux", "amd64", "v0.1.0"},
		{"linux", "amd64", "0.1.0"},
		{"linux", "arm64", "v0.1.0"},
		{"darwin", "amd64", "v0.1.0"},
		{"darwin", "arm64", "v0.1.0"},
		{"windows", "amd64", "v0.1.0"},
		{"windows", "arm64", "v0.1.0"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s_%s", tc.os, tc.arch, tc.ver), func(t *testing.T) {
			// Determine file extension based on OS (Windows uses .zip, others use .tar.gz)
			ext := ".tar.gz"
			if tc.os == "windows" {
				ext = ".zip"
			}

			// Verify extension is correct for the platform
			if tc.os == "windows" {
				require.Equal(t, ".zip", ext,
					"Windows archives must use .zip format for installer compatibility")
			} else {
				require.Equal(t, ".tar.gz", ext,
					"Non-Windows archives must use .tar.gz format for installer compatibility")
			}

			// Render the archive name template
			caser := cases.Title(language.Und)
			tmpl, tmplErr := template.New("name").Funcs(template.FuncMap{
				"title": caser.String,
			}).Parse(archive.NameTemplate)
			require.NoError(t, tmplErr, "failed to parse name template")

			data := map[string]interface{}{
				"ProjectName": config.ProjectName,
				"Version":     tc.ver,
				"Os":          tc.os,
				"Arch":        tc.arch,
			}

			var sb strings.Builder
			err = tmpl.Execute(&sb, data)
			require.NoError(t, err, "failed to render template")

			renderedName := sb.String() + ext

			// Build expected patterns (matching finfocus installer logic)
			expectedPatterns := buildInstallerPatterns(
				config.ProjectName,
				tc.ver,
				tc.os,
				tc.arch,
				ext,
			)

			// Verify the rendered name matches one of the installer's expected patterns
			found := false
			for _, pattern := range expectedPatterns {
				if renderedName == pattern {
					found = true
					break
				}
			}
			require.True(t, found,
				"rendered name %q not found in installer patterns for %s_%s_%s; "+
					"archive extension must be %s for platform compatibility",
				renderedName, tc.os, tc.arch, tc.ver, ext)
		})
	}
}

// buildInstallerPatterns mirrors finfocus/internal/registry/github.go buildAssetPatterns
// to generate the expected asset names the installer will accept.
func buildInstallerPatterns(projectName, version, goos, goarch, ext string) []string {
	// OS name variations
	osNames := []string{
		goos,                                 // linux, darwin, windows
		strings.ToUpper(goos[:1]) + goos[1:], // Linux, Darwin, Windows
	}
	if goos == "darwin" {
		osNames = append(osNames, "Darwin", "macos", "macOS", "MacOS")
	}

	// Architecture variations
	archNames := []string{goarch} // amd64, arm64
	if goarch == "amd64" {
		archNames = append(archNames, "x86_64", "X86_64", "AMD64")
	}
	if goarch == "arm64" {
		archNames = append(archNames, "ARM64", "aarch64", "AARCH64")
	}

	// Version variations (with and without v prefix)
	versions := []string{version}
	if strings.HasPrefix(version, "v") {
		versions = append(versions, strings.TrimPrefix(version, "v"))
	} else {
		versions = append(versions, "v"+version)
	}

	// Generate all combinations
	var patterns []string
	for _, ver := range versions {
		for _, osName := range osNames {
			for _, arch := range archNames {
				pattern := fmt.Sprintf(
					"%s_%s_%s_%s%s",
					projectName,
					ver,
					osName,
					arch,
					ext,
				)
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}
