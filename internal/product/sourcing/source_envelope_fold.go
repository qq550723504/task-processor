package sourcing

import (
	"regexp"
	"strings"
)

// MaxSourceEnvelopeCollectionItems bounds every envelope collection. Source
// adapters report one missing fact per instance (variant, price fact, ...), so an
// unbounded product would scale the collection with the variant count and hit
// that fixed bound. Folding keeps the recorded facts bounded by the envelope
// schema while retaining how many instances each folded class covered.

var (
	envelopeBracketIndex = regexp.MustCompile(`\[[0-9]+\]`)
	envelopeDotIndex     = regexp.MustCompile(`\.[0-9]+`)
)

// foldSourceEnvelopeRepeats collapses repeated per-instance facts and warnings
// into one schema-pattern entry carrying the occurrence count. Single-occurrence
// entries are returned unchanged so existing evidence keeps its exact shape.
func foldSourceEnvelopeRepeats(envelope *SourceEnvelope) {
	if envelope == nil {
		return
	}
	envelope.MissingFacts = foldMissingFacts(envelope.MissingFacts)
	envelope.Warnings = foldWarnings(envelope.Warnings)
}

// foldFieldPattern replaces instance indexes with an empty index so both the
// server spelling (variants.3.sku) and the client spelling (variants[0].sku)
// collapse to the same schema pattern (variants[].sku).
func foldFieldPattern(field string) string {
	if !strings.ContainsAny(field, ".[") {
		return field
	}
	return envelopeDotIndex.ReplaceAllString(envelopeBracketIndex.ReplaceAllString(field, "[]"), "[]")
}

func foldMissingFacts(facts []MissingFact) []MissingFact {
	if len(facts) < 2 {
		return facts
	}
	type key struct{ pattern, reason string }
	type group struct {
		count int
		first MissingFact
	}
	order := make([]key, 0, len(facts))
	groups := make(map[key]*group, len(facts))
	for _, fact := range facts {
		k := key{pattern: foldFieldPattern(fact.Field), reason: fact.Reason}
		entry, ok := groups[k]
		if !ok {
			entry = &group{first: fact}
			groups[k] = entry
			order = append(order, k)
		}
		entry.count++
	}
	folded := make([]MissingFact, 0, len(order))
	for _, k := range order {
		if entry := groups[k]; entry.count == 1 {
			folded = append(folded, entry.first)
		} else {
			folded = append(folded, MissingFact{Field: k.pattern, Reason: k.reason, Count: entry.count})
		}
	}
	return folded
}

func foldWarnings(warnings []SourceWarning) []SourceWarning {
	if len(warnings) < 2 {
		return warnings
	}
	type key struct{ code, pattern, message string }
	type group struct {
		count int
		first SourceWarning
	}
	order := make([]key, 0, len(warnings))
	groups := make(map[key]*group, len(warnings))
	for _, warning := range warnings {
		k := key{code: warning.Code, pattern: foldFieldPattern(warning.Field), message: warning.Message}
		entry, ok := groups[k]
		if !ok {
			entry = &group{first: warning}
			groups[k] = entry
			order = append(order, k)
		}
		entry.count++
	}
	folded := make([]SourceWarning, 0, len(order))
	for _, k := range order {
		if entry := groups[k]; entry.count == 1 {
			folded = append(folded, entry.first)
		} else {
			folded = append(folded, SourceWarning{Code: k.code, Message: k.message, Field: k.pattern, Count: entry.count})
		}
	}
	return folded
}
