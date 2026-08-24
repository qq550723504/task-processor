package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintUsagePreservesSingleTrailingNewline(t *testing.T) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	printUsage()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	os.Stdout = original

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read usage output: %v", err)
	}
	if strings.HasSuffix(string(output), "\n\n") {
		t.Fatalf("usage output has redundant trailing newline")
	}
	if !strings.HasSuffix(string(output), "\n") {
		t.Fatalf("usage output must end with a newline")
	}
}
