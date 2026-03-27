package amazonlisting

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	ipBrandTerms = []string{
		"nike", "adidas", "apple", "samsung", "dyson", "lego", "disney", "marvel",
		"pokemon", "gucci", "chanel", "rolex", "sony", "nintendo", "sanrio", "hello kitty",
	}
	ipHighRiskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcompatible with\b`),
		regexp.MustCompile(`(?i)\breplacement for\b`),
		regexp.MustCompile(`(?i)\binspired by\b`),
		regexp.MustCompile(`(?i)\blike\s+(nike|adidas|apple|samsung|dyson|lego|disney|marvel|pokemon|gucci|chanel|rolex|sony|nintendo|sanrio)\b`),
	}
	ipMediumRiskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bfor\s+(nike|adidas|apple|samsung|dyson|lego|disney|marvel|pokemon|gucci|chanel|rolex|sony|nintendo|sanrio)\b`),
	}
)

func assessContentIPRisk(req *GenerateRequest, draft *AmazonListingDraft) *IPRiskReport {
	if draft == nil {
		return nil
	}

	texts := collectIPRiskTexts(draft)
	if len(texts) == 0 {
		return nil
	}

	ownedBrands := collectOwnedBrands(req, draft)
	reasons := make([]string, 0)
	level := "low"
	score := 0.0

	for _, text := range texts {
		lower := strings.ToLower(text)
		for _, term := range ipBrandTerms {
			if !strings.Contains(lower, term) || ownedBrands[term] {
				continue
			}
			reasons = append(reasons, fmt.Sprintf("content references protected brand term %q", term))
			score += 0.35
			if level == "low" {
				level = "medium"
			}
		}
		for _, pattern := range ipHighRiskPatterns {
			if pattern.MatchString(text) {
				reasons = append(reasons, fmt.Sprintf("content contains high-risk compatibility or imitation phrase %q", pattern.FindString(text)))
				score += 0.55
				level = "high"
			}
		}
		for _, pattern := range ipMediumRiskPatterns {
			if pattern.MatchString(text) {
				reasons = append(reasons, fmt.Sprintf("content contains medium-risk phrase %q", pattern.FindString(text)))
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
	return &IPRiskReport{
		Level:   level,
		Score:   score,
		Reasons: reasons,
	}
}

func collectIPRiskTexts(draft *AmazonListingDraft) []string {
	values := make([]string, 0, 2+len(draft.BulletPoints)+len(draft.SearchTerms))
	if strings.TrimSpace(draft.Title) != "" {
		values = append(values, draft.Title)
	}
	if strings.TrimSpace(draft.Description) != "" {
		values = append(values, draft.Description)
	}
	values = append(values, draft.BulletPoints...)
	values = append(values, draft.SearchTerms...)
	return values
}

func collectOwnedBrands(req *GenerateRequest, draft *AmazonListingDraft) map[string]bool {
	values := map[string]bool{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			values[value] = true
		}
	}
	if req != nil {
		add(req.BrandHint)
	}
	if draft != nil {
		add(draft.Brand)
		if draft.Attributes != nil {
			add(draft.Attributes["brand"])
		}
	}
	return values
}
