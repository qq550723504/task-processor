package generation

import "testing"

func TestExecutionQualityLabel(t *testing.T) {
	tests := map[string]string{
		"renderer_output": "Renderer Output",
		" missing ":       "Missing",
		"unknown":         "",
	}

	for value, want := range tests {
		if got := ExecutionQualityLabel(value); got != want {
			t.Fatalf("ExecutionQualityLabel(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestQualityGrade(t *testing.T) {
	tests := map[string]string{
		"renderer_output": "ideal",
		"pipeline_output": "source_backed",
		"missing":         "missing",
		"unknown":         "",
	}

	for value, want := range tests {
		if got := QualityGrade(value); got != want {
			t.Fatalf("QualityGrade(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestQualityGradeLabel(t *testing.T) {
	if got := QualityGradeLabel("source_backed"); got != "Source Backed" {
		t.Fatalf("QualityGradeLabel() = %q, want Source Backed", got)
	}
}
