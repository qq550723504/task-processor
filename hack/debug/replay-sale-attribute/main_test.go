package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDebugFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "debug.json")
	content := `{
  "task_id": "8213798",
  "product_id": "B07N14BP26",
  "system_prompt": "sys",
  "user_prompt": "usr",
  "response": "resp",
  "finish_reason": "stop",
  "model": "gemini-2.5-flash",
  "tokens_used": 123,
  "is_truncated": false
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write debug file: %v", err)
	}

	data, err := loadDebugFile(path)
	if err != nil {
		t.Fatalf("loadDebugFile() error = %v", err)
	}

	if data.TaskID != "8213798" {
		t.Fatalf("TaskID = %q, want %q", data.TaskID, "8213798")
	}
	if data.SystemPrompt != "sys" {
		t.Fatalf("SystemPrompt = %q, want %q", data.SystemPrompt, "sys")
	}
	if data.UserPrompt != "usr" {
		t.Fatalf("UserPrompt = %q, want %q", data.UserPrompt, "usr")
	}
}

func TestIsLikelyTruncatedResponse(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"variants\":[{\"asin\":\"B0FDCR3HRC\",\"quantity\":1"
	if !isLikelyTruncatedResponse(content, "stop") {
		t.Fatalf("isLikelyTruncatedResponse() = false, want true")
	}
}

func TestAnalyzeReplayResponse_ValidJSONAndMatchingVariantCount(t *testing.T) {
	t.Parallel()

	userPrompt := "Task: generate SHEIN sale attributes for 20 Amazon products.\n\nThis is a multi-variant product. Generate 20 variants."
	content := "```json\n{\"saleAttributes\":[],\"variants\":[{\"asin\":\"A1\"},{\"asin\":\"A2\"}]}\n```"

	analysis := analyzeReplayResponse(userPrompt, content)

	if !analysis.JSONValid {
		t.Fatalf("JSONValid = false, want true")
	}
	if analysis.ExpectedVariantCount != 20 {
		t.Fatalf("ExpectedVariantCount = %d, want 20", analysis.ExpectedVariantCount)
	}
	if analysis.VariantCount != 2 {
		t.Fatalf("VariantCount = %d, want 2", analysis.VariantCount)
	}
	if analysis.VariantCountMatches {
		t.Fatalf("VariantCountMatches = true, want false")
	}
}

func TestAnalyzeReplayResponse_InvalidJSON(t *testing.T) {
	t.Parallel()

	analysis := analyzeReplayResponse("Generate 3 variants.", "```json\n{\"variants\":[{\"asin\":\"A1\"}")
	if analysis.JSONValid {
		t.Fatalf("JSONValid = true, want false")
	}
	if analysis.ParseError == "" {
		t.Fatalf("ParseError = empty, want non-empty")
	}
}
