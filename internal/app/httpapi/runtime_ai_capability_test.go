package httpapi

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func TestBuildAICapabilityRuntimeDepsKeepsLegacyStudioModeDependencyFree(t *testing.T) {
	deps, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "legacy"},
	}, logrus.New())
	if err != nil {
		t.Fatalf("buildAICapabilityRuntimeDeps() error = %v", err)
	}
	if deps.invocationRecorder != nil || deps.asyncJobStore != nil || len(deps.closers) != 0 {
		t.Fatalf("legacy Studio mode unexpectedly built persistence dependencies: %#v", deps)
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseOutsideLegacyStudioMode(t *testing.T) {
	for _, mode := range []string{"shadow", "active", "SHADOW", "Active"} {
		t.Run(mode, func(t *testing.T) {
			_, err := buildAICapabilityRuntimeDeps(&config.Config{
				AICapability: config.AICapabilityConfig{StudioImageRoutingMode: mode},
			}, logrus.New())
			if err == nil || !strings.Contains(err.Error(), "AI capability") {
				t.Fatalf("error = %v, want missing AI capability database", err)
			}
		})
	}
}
