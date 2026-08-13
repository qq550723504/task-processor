package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type codeHealthConfig struct {
	DeadcodeVersion string   `json:"deadcode_version"`
	KnipVersion     string   `json:"knip_version"`
	JscpdVersion    string   `json:"jscpd_version"`
	RootPatterns    []string `json:"root_patterns"`
	TargetGOOS      []string `json:"target_goos"`
	OutputRoot      string   `json:"output_root"`
	ClonePaths      []string `json:"clone_paths"`
	CloneIgnore     []string `json:"clone_ignore"`
}

func TestCodeHealthAuditConfigPinsScopeAndTools(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "scripts", "code-health-audit.config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg codeHealthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.DeadcodeVersion != "v0.48.0" || cfg.KnipVersion != "6.32.2" || cfg.JscpdVersion != "5.0.14" {
		t.Fatalf("unexpected analyzer versions: %+v", cfg)
	}
	for _, required := range []string{"./cmd/...", "./scripts/..."} {
		if !slices.Contains(cfg.RootPatterns, required) {
			t.Errorf("missing root %s", required)
		}
	}
	for _, goos := range []string{"windows", "linux"} {
		if !slices.Contains(cfg.TargetGOOS, goos) {
			t.Errorf("missing GOOS %s", goos)
		}
	}
	if cfg.OutputRoot != ".local/code-health" {
		t.Errorf("unsafe output root %q", cfg.OutputRoot)
	}
	for _, forbidden := range []string{"internal/**", "cmd/**", "scripts/**", "tools/**", "web/listingkit-ui/src/**"} {
		if slices.Contains(cfg.CloneIgnore, forbidden) {
			t.Errorf("broad clone exclusion hides source scope: %q", forbidden)
		}
	}
}

func TestCodeHealthAuditRunnerIsReadOnlyAndUsesModuleMode(t *testing.T) {
	runner, err := os.ReadFile(filepath.Join("..", "scripts", "code-health-audit.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(runner)
	for _, required := range []string{
		"ProcessStartInfo",
		"ArgumentList",
		"GOFLAGS",
		"-mod=",
		"Mode -in @(\"All\", \"Go\", \"Verify\")",
		"knip reported",
		"} finally {",
		"go test",
		"manifest.json",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("runner missing required safety/verification contract %q", required)
		}
	}
}
