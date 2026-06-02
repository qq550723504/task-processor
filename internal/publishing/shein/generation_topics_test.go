package shein

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildSheinGenerationPolicySummary_DeduplicatesAndPrioritizesTopics(t *testing.T) {
	summary := buildSheinGenerationPolicySummary([]string{
		"food",
		"children",
		"children",
		"knives",
		"unknown",
		" food ",
	})

	expectedDirectives := []string{
		"Do not mention children, babies, or age-specific users.",
		"Do not mention food, meals, or edible usage scenarios.",
		"Do not mention knives, blades, or sharp-tool contexts.",
	}
	for _, expected := range expectedDirectives {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary = %q, want directive %q", summary, expected)
		}
	}
	if strings.Count(summary, "Do not mention children, babies, or age-specific users.") != 1 {
		t.Fatalf("summary = %q, want deduplicated children directive", summary)
	}

	childrenIndex := strings.Index(summary, expectedDirectives[0])
	foodIndex := strings.Index(summary, expectedDirectives[1])
	knivesIndex := strings.Index(summary, expectedDirectives[2])
	if !(childrenIndex >= 0 && foodIndex > childrenIndex && knivesIndex > foodIndex) {
		t.Fatalf("summary = %q, want directives ordered by priority", summary)
	}
}

func TestBuildSheinGenerationPolicySummary_CoversEachKnownTopicBeforeExtraDirectives(t *testing.T) {
	selected := []string{"children", "baby", "food", "meals", "knives"}
	summary := buildSheinGenerationPolicySummary(selected)

	for _, key := range selected {
		definition := sheinGenerationTopicDefinitions[key]
		if len(definition.PromptDirectives) == 0 {
			t.Fatalf("topic %q has no prompt directives", key)
		}
		if !strings.Contains(summary, definition.PromptDirectives[0]) {
			t.Fatalf("summary = %q, want first directive for topic %q", summary, key)
		}
	}
}

func TestAssembleSheinGenerationPolicySummary_PrimaryCoverageComesBeforeExtras(t *testing.T) {
	definitions := []GenerationTopicDefinition{
		{
			Key:      "alpha",
			Priority: 10,
			PromptDirectives: []string{
				"alpha-primary",
				"alpha-extra",
			},
		},
		{
			Key:      "beta",
			Priority: 20,
			PromptDirectives: []string{
				"beta-primary",
			},
		},
	}

	summary := assembleSheinGenerationPolicySummary(definitions, 3, 200)

	alphaPrimary := strings.Index(summary, "alpha-primary")
	betaPrimary := strings.Index(summary, "beta-primary")
	alphaExtra := strings.Index(summary, "alpha-extra")
	if alphaPrimary < 0 || betaPrimary < 0 || alphaExtra < 0 {
		t.Fatalf("summary = %q, want both primary directives and the extra directive", summary)
	}
	if !(alphaPrimary < betaPrimary && betaPrimary < alphaExtra) {
		t.Fatalf("summary = %q, want all primary directives before extras", summary)
	}
}

func TestBuildSheinGenerationPolicySummary_UsesRuneLimitSafely(t *testing.T) {
	definitions := []GenerationTopicDefinition{
		{
			Key:              "unicode-a",
			Priority:         10,
			PromptDirectives: []string{"甲甲甲", "ignored optional directive"},
		},
		{
			Key:              "unicode-b",
			Priority:         20,
			PromptDirectives: []string{"乙乙乙"},
		},
	}

	summary := assembleSheinGenerationPolicySummary(definitions, 5, 7)
	if summary != "甲甲甲\n乙乙乙" {
		t.Fatalf("summary = %q, want unicode-safe inclusion up to rune limit", summary)
	}
	if utf8.RuneCountInString(summary) != 7 {
		t.Fatalf("summary rune count = %d, want 7", utf8.RuneCountInString(summary))
	}
}

func TestResolveSheinGenerationTopicKeys_ReturnsKnownAndUnknownKeys(t *testing.T) {
	known, unknown := ResolveSheinGenerationTopicKeys([]string{
		" food ",
		"unknown",
		"children",
		"UNKNOWN",
		"",
		"children",
	})

	if len(known) != 2 {
		t.Fatalf("known topic count = %d, want 2", len(known))
	}
	if known[0].Key != "children" || known[1].Key != "food" {
		t.Fatalf("known topics = %#v, want ordered children then food", known)
	}
	if got, want := unknown, []string{"unknown"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unknown topics = %#v, want %#v", got, want)
	}
}

func TestResolveSheinGenerationTopics_ReturnsKnownAndUnknownKeys(t *testing.T) {
	resolution := ResolveSheinGenerationTopics([]string{"food", "unknown"})
	if len(resolution.Known) != 1 || resolution.Known[0].Key != "food" {
		t.Fatalf("resolution.Known = %#v, want only food", resolution.Known)
	}
	if got, want := resolution.Unknown, []string{"unknown"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("resolution.Unknown = %#v, want %#v", got, want)
	}
}

func TestSheinGenerationTopicDefinitions_LexiconSanity(t *testing.T) {
	for key, definition := range sheinGenerationTopicDefinitions {
		for language, words := range definition.LexiconByLanguage {
			if len(words) == 0 {
				t.Fatalf("topic %q language %q has empty lexicon", key, language)
			}
			seen := make(map[string]struct{}, len(words))
			for _, word := range words {
				trimmed := strings.TrimSpace(word)
				if trimmed == "" {
					t.Fatalf("topic %q language %q contains empty lexicon entry", key, language)
				}
				normalized := strings.ToLower(trimmed)
				if _, exists := seen[normalized]; exists {
					t.Fatalf("topic %q language %q contains duplicate lexicon entry %q", key, language, trimmed)
				}
				seen[normalized] = struct{}{}
			}
		}
	}
}
