package httpapi

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func TestBuildAICapabilityRuntimeDepsKeepsLegacyModeDependencyFree(t *testing.T) {
	deps, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "legacy"},
	}, logrus.New())
	if err != nil {
		t.Fatalf("buildAICapabilityRuntimeDeps() error = %v", err)
	}
	if deps.invocationRecorder != nil {
		t.Fatal("expected legacy mode to omit invocation recorder")
	}
	if len(deps.closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(deps.closers))
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseOutsideLegacy(t *testing.T) {
	for _, mode := range []string{"shadow", "active"} {
		t.Run(mode, func(t *testing.T) {
			_, err := buildAICapabilityRuntimeDeps(&config.Config{
				AICapability: config.AICapabilityConfig{StudioImageRoutingMode: mode},
			}, logrus.New())
			if err == nil {
				t.Fatal("expected missing database error")
			}
			if !strings.Contains(err.Error(), "AI capability") {
				t.Fatalf("error = %q, want AI capability resource context", err)
			}
		})
	}
}
