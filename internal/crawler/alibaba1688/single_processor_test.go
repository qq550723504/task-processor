package alibaba1688

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestSingleProcessorCreatesFreshPublicBrowserManagerPerAttempt(t *testing.T) {
	processor := &SingleProcessor{config: config.NewDefaultConfig()}

	first := processor.newPublicBrowserManager()
	second := processor.newPublicBrowserManager()

	if first == nil || second == nil {
		t.Fatal("public browser manager factory returned nil")
	}
	if first == second {
		t.Fatal("public browser attempts must not share a browser manager")
	}
}
