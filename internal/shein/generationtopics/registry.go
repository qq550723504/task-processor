package generationtopics

import (
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	SheinGenerationPolicyMaxDirectives = 5
	SheinGenerationPolicyMaxChars      = 600
)

type Definition struct {
	Key               string
	PromptDirectives  []string
	LexiconByLanguage map[string][]string
	Priority          int
}

type Resolution struct {
	Known   []Definition
	Unknown []string
}

type DefinitionOverlay struct {
	Enabled           bool
	PromptDirectives  []string
	LexiconByLanguage map[string][]string
}

var sheinGenerationTopicDefinitions = map[string]Definition{
	"children": {
		Key:      "children",
		Priority: 10,
		PromptDirectives: []string{
			"Do not mention children, babies, or age-specific users.",
		},
		LexiconByLanguage: map[string][]string{
			"en": {"child", "children", "kid", "kids"},
			"zh": {"儿童", "孩童", "小朋友"},
		},
	},
	"baby": {
		Key:      "baby",
		Priority: 20,
		PromptDirectives: []string{
			"Do not mention babies, newborns, or infant-specific usage.",
			"Avoid nursery, stroller, feeding, or infant-care positioning.",
		},
		LexiconByLanguage: map[string][]string{
			"en": {"baby", "babies", "newborn", "infant", "crib"},
			"zh": {"婴儿", "新生儿", "宝宝", "婴童"},
		},
	},
	"food": {
		Key:      "food",
		Priority: 30,
		PromptDirectives: []string{
			"Do not mention food, meals, or edible usage scenarios.",
		},
		LexiconByLanguage: map[string][]string{
			"en": {"food", "snack", "snacks", "edible"},
			"zh": {"食品", "食物", "零食", "甜点"},
		},
	},
	"meals": {
		Key:      "meals",
		Priority: 40,
		PromptDirectives: []string{
			"Do not frame the product around breakfast, lunch, dinner, or serving moments.",
			"Avoid dining-table, recipe, or menu-oriented storytelling.",
		},
		LexiconByLanguage: map[string][]string{
			"en": {"meal", "meals", "breakfast", "lunch", "dinner"},
			"zh": {"早餐", "午餐", "晚餐", "餐点", "食谱"},
		},
	},
	"knives": {
		Key:      "knives",
		Priority: 50,
		PromptDirectives: []string{
			"Do not mention knives, blades, or sharp-tool contexts.",
			"Avoid cutlery, weapon-like, slicing, or stabbing language.",
		},
		LexiconByLanguage: map[string][]string{
			"en": {"knife", "knives", "blade", "blades", "dagger"},
			"zh": {"刀具", "刀刃", "菜刀", "匕首"},
		},
	},
}

func SheinGenerationTopicDefinitions() map[string]Definition {
	out := make(map[string]Definition, len(sheinGenerationTopicDefinitions))
	for key, definition := range sheinGenerationTopicDefinitions {
		out[key] = cloneDefinition(definition)
	}
	return out
}

func ResolveSheinTopics(topicKeys []string) Resolution {
	known, unknown := ResolveSheinTopicKeys(topicKeys)
	return Resolution{
		Known:   known,
		Unknown: unknown,
	}
}

func ResolveSheinTopicKeys(topicKeys []string) ([]Definition, []string) {
	if len(topicKeys) == 0 {
		return nil, nil
	}

	seenKnown := make(map[string]struct{}, len(topicKeys))
	seenUnknown := make(map[string]struct{}, len(topicKeys))
	known := make([]Definition, 0, len(topicKeys))
	unknown := make([]string, 0)

	for _, topicKey := range topicKeys {
		normalized := NormalizeKey(topicKey)
		if normalized == "" {
			continue
		}
		if definition, ok := sheinGenerationTopicDefinitions[normalized]; ok {
			if _, exists := seenKnown[definition.Key]; exists {
				continue
			}
			seenKnown[definition.Key] = struct{}{}
			known = append(known, cloneDefinition(definition))
			continue
		}
		if _, exists := seenUnknown[normalized]; exists {
			continue
		}
		seenUnknown[normalized] = struct{}{}
		unknown = append(unknown, normalized)
	}

	sort.Slice(known, func(i, j int) bool {
		if known[i].Priority == known[j].Priority {
			return known[i].Key < known[j].Key
		}
		return known[i].Priority < known[j].Priority
	})

	return known, unknown
}

func BuildSheinPolicySummary(topicKeys []string) string {
	known, _ := ResolveSheinTopicKeys(topicKeys)
	return AssembleSheinPolicySummary(
		known,
		SheinGenerationPolicyMaxDirectives,
		SheinGenerationPolicyMaxChars,
	)
}

func ResolveSheinTopicDefinitionsWithOverlay(topicKeys []string, overlayForKey func(string) (DefinitionOverlay, error)) []Definition {
	definitions, _ := ResolveSheinTopicKeys(topicKeys)
	if len(definitions) == 0 || overlayForKey == nil {
		return definitions
	}

	merged := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		overlay, err := overlayForKey(definition.Key)
		if err != nil || !overlay.Enabled {
			merged = append(merged, definition)
			continue
		}
		merged = append(merged, MergeDefinition(
			definition,
			overlay.PromptDirectives,
			overlay.LexiconByLanguage,
		))
	}
	return merged
}

func AssembleSheinPolicySummary(definitions []Definition, maxDirectives int, maxChars int) string {
	if len(definitions) == 0 || maxDirectives <= 0 || maxChars <= 0 {
		return ""
	}

	directives := make([]string, 0, maxDirectives)
	requiredDirectives := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if directive := firstNonEmptyDirective(definition.PromptDirectives); directive != "" {
			requiredDirectives = append(requiredDirectives, directive)
		}
	}
	for _, directive := range requiredDirectives {
		if len(directives) >= maxDirectives {
			return strings.Join(directives, "\n")
		}
		if !canAppendDirective(directives, directive, maxChars) {
			return strings.Join(directives, "\n")
		}
		directives = append(directives, directive)
	}

	for _, definition := range definitions {
		remaining := remainingDirectives(definition.PromptDirectives)
		for _, directive := range remaining {
			if len(directives) >= maxDirectives {
				return strings.Join(directives, "\n")
			}
			if !canAppendDirective(directives, directive, maxChars) {
				return strings.Join(directives, "\n")
			}
			directives = append(directives, directive)
		}
	}

	return strings.Join(directives, "\n")
}

func CollectSheinTopicLexicons(topicKeys []string) map[string][]string {
	definitions, _ := ResolveSheinTopicKeys(topicKeys)
	return CollectLexiconsFromDefinitions(definitions)
}

func CollectLexiconsFromDefinitions(definitions []Definition) map[string][]string {
	if len(definitions) == 0 {
		return nil
	}
	wordsByLanguage := make(map[string][]string)
	seenByLanguage := make(map[string]map[string]struct{})
	for _, definition := range definitions {
		for language, words := range definition.LexiconByLanguage {
			normalizedLanguage := strings.TrimSpace(strings.ToLower(language))
			if normalizedLanguage == "" {
				continue
			}
			seen := seenByLanguage[normalizedLanguage]
			if seen == nil {
				seen = make(map[string]struct{})
				seenByLanguage[normalizedLanguage] = seen
			}
			for _, word := range words {
				word = strings.TrimSpace(word)
				if word == "" {
					continue
				}
				key := strings.ToLower(word)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				wordsByLanguage[normalizedLanguage] = append(wordsByLanguage[normalizedLanguage], word)
			}
		}
	}
	return wordsByLanguage
}

func MergeDefinition(definition Definition, additionalDirectives []string, additionalLexicon map[string][]string) Definition {
	merged := cloneDefinition(definition)
	if len(additionalDirectives) > 0 {
		merged.PromptDirectives = mergeStringList(merged.PromptDirectives, additionalDirectives)
	}
	if len(additionalLexicon) > 0 {
		merged.LexiconByLanguage = mergeLexiconMap(merged.LexiconByLanguage, additionalLexicon)
	}
	return merged
}

func NormalizeKey(topicKey string) string {
	return strings.TrimSpace(strings.ToLower(topicKey))
}

func cloneDefinition(definition Definition) Definition {
	cloned := definition
	if len(definition.PromptDirectives) > 0 {
		cloned.PromptDirectives = append([]string(nil), definition.PromptDirectives...)
	}
	if len(definition.LexiconByLanguage) > 0 {
		cloned.LexiconByLanguage = make(map[string][]string, len(definition.LexiconByLanguage))
		for language, words := range definition.LexiconByLanguage {
			cloned.LexiconByLanguage[language] = append([]string(nil), words...)
		}
	}
	return cloned
}

func firstNonEmptyDirective(directives []string) string {
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive != "" {
			return directive
		}
	}
	return ""
}

func remainingDirectives(directives []string) []string {
	firstSeen := false
	remaining := make([]string, 0, len(directives))
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive == "" {
			continue
		}
		if !firstSeen {
			firstSeen = true
			continue
		}
		remaining = append(remaining, directive)
	}
	return remaining
}

func canAppendDirective(existing []string, directive string, maxChars int) bool {
	if strings.TrimSpace(directive) == "" {
		return false
	}
	candidate := append(append([]string(nil), existing...), directive)
	return utf8.RuneCountInString(strings.Join(candidate, "\n")) <= maxChars
}

func mergeStringList(base []string, additions []string) []string {
	if len(base) == 0 && len(additions) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(additions))
	merged := make([]string, 0, len(base)+len(additions))
	for _, group := range [][]string{base, additions} {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			key := strings.ToLower(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func mergeLexiconMap(base map[string][]string, additions map[string][]string) map[string][]string {
	if len(base) == 0 && len(additions) == 0 {
		return nil
	}
	merged := cloneDefinition(Definition{LexiconByLanguage: base}).LexiconByLanguage
	if merged == nil {
		merged = make(map[string][]string)
	}
	for language, words := range additions {
		normalizedLanguage := NormalizeKey(language)
		if normalizedLanguage == "" {
			continue
		}
		merged[normalizedLanguage] = mergeStringList(merged[normalizedLanguage], words)
		if len(merged[normalizedLanguage]) == 0 {
			delete(merged, normalizedLanguage)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
