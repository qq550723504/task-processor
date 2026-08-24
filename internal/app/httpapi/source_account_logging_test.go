package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestSourceAccountRepositoryStartupLogDoesNotIncludeRawError(t *testing.T) {
	source, err := os.ReadFile("composition_builder.go")
	if err != nil {
		t.Fatalf("read composition_builder.go: %v", err)
	}
	if strings.Contains(string(source), "WithError(sourceErr)") {
		t.Fatal("source-account repository startup logging must not include raw sourceErr")
	}
}
