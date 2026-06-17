package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemporalBoundaryDocumentDefinesStableReviewRules(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "temporal-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := []string{
		"# Temporal Boundaries",
		"HTTP API",
		"service facade",
		"workflow runtime",
		"RabbitMQ",
		"Review Questions",
	}
	for _, phrase := range required {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q so Temporal changes have a stable review boundary", path, phrase)
		}
	}
}
