package productenrich

import (
	"context"
	"testing"
)

func TestResultValidator_CheckImageConsistency(t *testing.T) {
	v := NewResultValidator()

	cases := []struct {
		name         string
		inputImages  []string
		resultImages []string
		want         bool
	}{
		{"no input images always ok", nil, nil, true},
		{"no input images with result", nil, []string{"a.jpg"}, true},
		{"result contains all input", []string{"a.jpg", "b.jpg"}, []string{"a.jpg", "b.jpg", "c.jpg"}, true},
		{"result missing one input", []string{"a.jpg", "b.jpg"}, []string{"a.jpg"}, false},
		{"result empty but input not", []string{"a.jpg"}, nil, false},
		{"exact match", []string{"a.jpg"}, []string{"a.jpg"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := v.CheckImageConsistency(tc.inputImages, tc.resultImages)
			if got != tc.want {
				t.Errorf("CheckImageConsistency = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResultValidator_CheckCompleteness(t *testing.T) {
	v := NewResultValidator()

	cases := []struct {
		name            string
		result          *ProductJSON
		wantErr         bool
		wantMissingReqs []string
	}{
		{"nil result returns error", nil, true, nil},
		{
			name: "complete product",
			result: &ProductJSON{
				Title:       "Test",
				Category:    []string{"Cat"},
				Description: "Desc",
			},
			wantMissingReqs: []string{},
		},
		{
			name:            "missing title",
			result:          &ProductJSON{Category: []string{"Cat"}, Description: "Desc"},
			wantMissingReqs: []string{"title"},
		},
		{
			name:            "missing all required",
			result:          &ProductJSON{},
			wantMissingReqs: []string{"title", "category", "description"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := v.CheckCompleteness(tc.result)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// 检查缺失的必需字段
			missing := make(map[string]bool, len(report.MissingRequired))
			for _, f := range report.MissingRequired {
				missing[f] = true
			}
			for _, f := range tc.wantMissingReqs {
				if !missing[f] {
					t.Errorf("expected %q in MissingRequired, got %v", f, report.MissingRequired)
				}
			}
			if len(tc.wantMissingReqs) == 0 && len(report.MissingRequired) != 0 {
				t.Errorf("expected no missing required fields, got %v", report.MissingRequired)
			}
		})
	}
}

func TestResultValidator_CheckKeywordMatch(t *testing.T) {
	v := NewResultValidator()

	cases := []struct {
		name      string
		input     string
		title     string
		desc      string
		wantAbove float64
	}{
		{"empty input always 1.0", "", "anything", "anything", 1.0},
		{"full match", "red shoes", "red shoes", "buy red shoes now", 0.9},
		{"no match", "blue hat", "green shirt", "cotton fabric", 0.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := v.CheckKeywordMatch(tc.input, tc.title, tc.desc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantAbove == 1.0 && score != 1.0 {
				t.Errorf("score = %.2f, want 1.0", score)
			}
			if tc.wantAbove == 0.0 && score > 0.1 {
				t.Errorf("score = %.2f, expected near 0", score)
			}
			if tc.wantAbove == 0.9 && score < 0.9 {
				t.Errorf("score = %.2f, want >= 0.9", score)
			}
		})
	}
}

func TestResultValidator_ValidateResult(t *testing.T) {
	ctx := context.Background()
	v := NewResultValidator()

	cases := []struct {
		name      string
		input     *ParsedInput
		result    *ProductJSON
		wantErr   bool
		wantValid bool
	}{
		{"nil input returns error", nil, &ProductJSON{}, true, false},
		{"nil result returns error", &ParsedInput{}, nil, true, false},
		{
			name:  "valid complete result",
			input: &ParsedInput{Images: []string{"a.jpg"}, Text: "product"},
			result: &ProductJSON{
				Title:       "Product",
				Category:    []string{"Cat"},
				Description: "A product",
				Images:      []string{"a.jpg"},
			},
			wantValid: true,
		},
		{
			name:   "missing required fields marks invalid",
			input:  &ParsedInput{},
			result: &ProductJSON{
				// Title, Category, Description all empty
			},
			wantValid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validation, err := v.ValidateResult(ctx, tc.input, tc.result)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if validation.IsValid != tc.wantValid {
				t.Errorf("IsValid = %v, want %v (issues: %v)", validation.IsValid, tc.wantValid, validation.Issues)
			}
		})
	}
}
