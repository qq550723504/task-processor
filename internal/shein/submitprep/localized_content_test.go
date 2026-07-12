package submitprep

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"task-processor/internal/shein/namelimit"
)

type fixedTranslation struct{ text string }

func (f fixedTranslation) Translate(string, string, string) (string, error) { return f.text, nil }

func TestBuildLocalizedTitleAndDescriptionAppliesNameLimits(t *testing.T) {
	t.Parallel()

	names, _, err := BuildLocalizedTitleAndDescription(
		context.Background(),
		"US",
		strings.Repeat("a", 20),
		"description",
		"",
		"",
		nil,
		nil,
		fixedTranslation{text: "一二三四五"},
		namelimit.Limits{"en": 12, "es": 3},
	)
	if err != nil {
		t.Fatalf("BuildLocalizedTitleAndDescription() error = %v", err)
	}
	for _, item := range names {
		maxLength, ok := map[string]int{"en": 12, "es": 3}[item.Language]
		if !ok {
			continue
		}
		if got := utf8.RuneCountInString(item.Name); got > maxLength {
			t.Errorf("%s name length = %d, want <= %d: %q", item.Language, got, maxLength, item.Name)
		}
	}
}
