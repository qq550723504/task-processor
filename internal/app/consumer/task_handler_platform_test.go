package consumer

import (
	"testing"

	"task-processor/internal/listingruntime"
)

func TestModelTaskFromRuntimeUsesTargetPlatformAndPreservesSourcePlatform(t *testing.T) {
	got := modelTaskFromRuntime(&listingruntime.ImportTask{
		ID:             9025239,
		Platform:       "amazon",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
	})

	if got.Platform != "shein" {
		t.Fatalf("model task platform = %q, want shein target", got.Platform)
	}
	if got.SourcePlatform != "amazon" {
		t.Fatalf("model task source platform = %q, want amazon source", got.SourcePlatform)
	}
}
