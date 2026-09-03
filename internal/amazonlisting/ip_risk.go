package amazonlisting

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	ipBrandTerms       = []string{"nike", "adidas", "apple", "samsung", "dyson", "lego", "disney", "marvel", "pokemon", "gucci", "chanel", "rolex", "sony", "nintendo", "sanrio", "hello kitty"}
	ipHighRiskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcompatible with\b`), regexp.MustCompile(`(?i)\breplacement for\b`), regexp.MustCompile(`(?i)\binspired by\b`),
		regexp.MustCompile(`(?i)\blike\s+(nike|adidas|apple|samsung|dyson|lego|disney|marvel|pokemon|gucci|chanel|rolex|sony|nintendo|sanrio)\b`),
	}
	ipMediumRiskPatterns = []*regexp.Regexp{regexp.MustCompile(`(?i)\bfor\s+(nike|adidas|apple|samsung|dyson|lego|disney|marvel|pokemon|gucci|chanel|rolex|sony|nintendo|sanrio)\b`)}
)

func assessContentIPRisk(req *GenerateRequest, draft *AmazonListingDraft) *IPRiskReport {
	if draft == nil {
		return nil
	}
	texts := make([]string, 0, 2+len(draft.BulletPoints)+len(draft.SearchTerms))
	if strings.TrimSpace(draft.Title) != "" {
		texts = append(texts, draft.Title)
	}
	if strings.TrimSpace(draft.Description) != "" {
		texts = append(texts, draft.Description)
	}
	texts = append(texts, draft.BulletPoints...)
	texts = append(texts, draft.SearchTerms...)
	owned := map[string]bool{}
	addOwned := func(value string) {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			owned[value] = true
		}
	}
	if req != nil {
		addOwned(req.BrandHint)
	}
	addOwned(draft.Brand)
	if draft.Attributes != nil {
		addOwned(draft.Attributes["brand"])
	}
	reasons := make([]string, 0)
	level := "low"
	score := 0.0
	for _, value := range texts {
		lower := strings.ToLower(value)
		for _, term := range ipBrandTerms {
			if strings.Contains(lower, term) && !owned[term] {
				reasons = append(reasons, fmt.Sprintf("content references protected brand term %q", term))
				score += 0.35
				if level == "low" {
					level = "medium"
				}
			}
		}
		for _, pattern := range ipHighRiskPatterns {
			if pattern.MatchString(value) {
				reasons = append(reasons, fmt.Sprintf("content contains high-risk compatibility or imitation phrase %q", pattern.FindString(value)))
				score += 0.55
				level = "high"
			}
		}
		for _, pattern := range ipMediumRiskPatterns {
			if pattern.MatchString(value) {
				reasons = append(reasons, fmt.Sprintf("content contains medium-risk phrase %q", pattern.FindString(value)))
				score += 0.25
				if level == "low" {
					level = "medium"
				}
			}
		}
	}
	reasons = uniqueSorted(reasons)
	if len(reasons) == 0 {
		return nil
	}
	if score > 1 {
		score = 1
	}
	return &IPRiskReport{Level: level, Score: score, Reasons: reasons}
}

func mergeListingIPRisk(existing, assessed *IPRiskReport) *IPRiskReport {
	if existing == nil {
		return assessed
	}
	if assessed == nil {
		return existing
	}
	merged := &IPRiskReport{Level: existing.Level, Score: existing.Score, Reasons: append([]string(nil), existing.Reasons...)}
	if riskRank(assessed.Level) > riskRank(merged.Level) {
		merged.Level = assessed.Level
	}
	if assessed.Score > merged.Score {
		merged.Score = assessed.Score
	}
	merged.Reasons = uniqueSorted(append(merged.Reasons, assessed.Reasons...))
	return merged
}

func riskRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
