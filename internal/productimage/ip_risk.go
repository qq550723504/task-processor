package productimage

import (
	"fmt"
	"strings"
)

func assessImageIPRisk(source *SourceBundle, audits []ImageAudit) *IPRiskReport {
	reasons := make([]string, 0)
	level := "low"
	score := 0.0

	if source != nil && source.ProductURL != "" && strings.Contains(strings.ToLower(source.ProductURL), "1688.com") {
		reasons = append(reasons, "image pipeline uses scraped 1688 source images")
		score += 0.15
		if level == "low" {
			level = "medium"
		}
	}

	for _, audit := range audits {
		lower := strings.ToLower(audit.ImageURL)
		if audit.HasLogo {
			reasons = append(reasons, describeImageIPRisk(audit, "image contains logo or watermark risk"))
			score += 0.35
			if level == "low" {
				level = "medium"
			}
		}
		if audit.HasOverlayText {
			reasons = append(reasons, describeImageIPRisk(audit, "image contains text overlay risk"))
			score += 0.2
			if level == "low" {
				level = "medium"
			}
		}
		if audit.HasPromoBadge {
			reasons = append(reasons, describeImageIPRisk(audit, "image contains promo badge risk"))
			score += 0.2
			if level == "low" {
				level = "medium"
			}
		}
		if containsAny(lower, "brand", "disney", "marvel", "pokemon", "lego", "nike", "adidas", "apple", "dyson", "hello-kitty", "sanrio") {
			reasons = append(reasons, describeImageIPRisk(audit, "image URL indicates branded or protected content risk"))
			score += 0.5
			level = "high"
		}
	}

	reasons = uniqueStrings(reasons)
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

func describeImageIPRisk(audit ImageAudit, message string) string {
	if object := strings.TrimSpace(audit.PrimaryObject); object != "" {
		return fmt.Sprintf("%s for %s", message, object)
	}
	return message
}
