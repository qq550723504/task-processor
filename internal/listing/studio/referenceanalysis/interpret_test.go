package referenceanalysis

import (
	"errors"
	"strings"
	"testing"
)

func TestInterpretRejectsEmptyInput(t *testing.T) {
	_, err := Interpret(nil)
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(nil) error = %v, want ErrNoInput", err)
	}
}

func TestInterpretRejectsWhitespaceOnlyInput(t *testing.T) {
	_, err := Interpret([]string{"  ", "\n"})
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(whitespace) error = %v, want ErrNoInput", err)
	}
}

func TestInterpretPreservesExactStructuredOutput(t *testing.T) {
	raw := `{"motif":"Retro Flowers","palette":["Cream","Cherry Red"],"composition":"Centered Badge","typography":"Old English","density":"Clean Layering","product_fit":"Vintage Streetwear"}`

	got, err := Interpret([]string{raw})
	if err != nil {
		t.Fatalf("Interpret() error = %v", err)
	}
	const wantBrief = "Reference style cues. Motif family: retro flowers. " +
		"Palette direction: cream, cherry red. " +
		"Composition family: centered composition, badge composition. " +
		"Typography feel: old english. Visual density: clean layering. " +
		"Product fit: vintage streetwear."
	const wantPrompt = "Create an original POD artwork with a commercially proven graphic style direction. " +
		"Motif direction: retro flowers. Palette direction: cream, cherry red. " +
		"Composition direction: centered composition, badge composition. " +
		"Typography feel: old english. Visual density: clean layering. " +
		"Product fit: vintage streetwear. Keep all graphics brand-neutral, " +
		"use fresh custom wording if text appears, avoid recognizable characters or people, " +
		"and use a clearly original layout."
	if got.StyleBrief != wantBrief {
		t.Fatalf("StyleBrief = %q, want %q", got.StyleBrief, wantBrief)
	}
	if got.SanitizedPrompt != wantPrompt {
		t.Fatalf("SanitizedPrompt = %q, want %q", got.SanitizedPrompt, wantPrompt)
	}
	if got.HadUnsafeInput || got.HadMalformedInput {
		t.Fatalf("flags = unsafe:%t malformed:%t, want false/false",
			got.HadUnsafeInput, got.HadMalformedInput)
	}
}

func TestInterpretPreservesPolicyFlags(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantContains  []string
		wantAbsent    []string
		wantUnsafe    bool
		wantMalformed bool
		wantErr       error
	}{
		{
			name:         "protected structured fields",
			raw:          `{"motif":"Hello Kitty bow","typography":"Old English","avoid":["Adidas trefoil logo","exact slogan Just Do It"]}`,
			wantContains: []string{"old english", "clearly original layout"},
			wantAbsent:   []string{"hello kitty", "adidas", "trefoil", "just do it"},
			wantUnsafe:   true,
		},
		{
			name:          "safe malformed cues",
			raw:           "distressed serif, clean layering, vintage streetwear",
			wantContains:  []string{"distressed serif", "clean layering", "vintage streetwear"},
			wantMalformed: true,
		},
		{
			name:    "no safe direction",
			raw:     `{"motif":"Hello Kitty","palette":["Nike"],"composition":"same exact layout","typography":"Taylor Swift signature quote","density":"Mickey portrait","product_fit":"Adidas logo","avoid":["Just Do It slogan"]}`,
			wantErr: ErrNoSafeDirection,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Interpret([]string{tt.raw})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Interpret() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			combined := strings.ToLower(got.StyleBrief + " " + got.SanitizedPrompt)
			for _, want := range tt.wantContains {
				if !strings.Contains(combined, want) {
					t.Fatalf("output = %q, want %q", combined, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(combined, absent) {
					t.Fatalf("output = %q, must not contain %q", combined, absent)
				}
			}
			if got.HadUnsafeInput != tt.wantUnsafe ||
				got.HadMalformedInput != tt.wantMalformed {
				t.Fatalf("flags = %t/%t, want %t/%t",
					got.HadUnsafeInput, got.HadMalformedInput,
					tt.wantUnsafe, tt.wantMalformed)
			}
		})
	}
}
