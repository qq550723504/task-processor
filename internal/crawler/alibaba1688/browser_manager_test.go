package alibaba1688

import (
	"task-processor/internal/core/config"
	"testing"
)

func TestResolveAlibaba1688UserDataDirUsesConfiguredValue(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Browser.UserDataDir = "./.local/tmp/browser-profiles/custom-1688"

	got := resolveAlibaba1688UserDataDir(cfg)

	if got != "./.local/tmp/browser-profiles/custom-1688" {
		t.Fatalf("expected configured user data dir, got %q", got)
	}
}

func TestResolveAlibaba1688UserDataDirUsesSharedDefault(t *testing.T) {
	cfg := config.NewDefaultConfig()

	got := resolveAlibaba1688UserDataDir(cfg)

	if got == "" {
		t.Fatal("expected non-empty default user data dir")
	}
}
