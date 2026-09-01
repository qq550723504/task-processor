package image

import (
	"fmt"
	"image"
	"reflect"
	"sort"
	"strings"
)

func IsWhiteBackground(source image.Image) bool {
	if source == nil || isNilImage(source) {
		return false
	}
	bounds := source.Bounds()
	if bounds.Empty() {
		return false
	}
	xInset := minInteger(2, (bounds.Dx()-1)/2)
	yInset := minInteger(2, (bounds.Dy()-1)/2)
	points := []image.Point{
		{X: bounds.Min.X + xInset, Y: bounds.Min.Y + yInset},
		{X: bounds.Max.X - 1 - xInset, Y: bounds.Min.Y + yInset},
		{X: bounds.Min.X + xInset, Y: bounds.Max.Y - 1 - yInset},
		{X: bounds.Max.X - 1 - xInset, Y: bounds.Max.Y - 1 - yInset},
	}
	unique := make(map[image.Point]struct{}, len(points))
	white := 0
	for _, point := range points {
		if _, exists := unique[point]; exists {
			continue
		}
		unique[point] = struct{}{}
		red, green, blue, _ := source.At(point.X, point.Y).RGBA()
		if red>>8 >= 240 && green>>8 >= 240 && blue>>8 >= 240 {
			white++
		}
	}
	return white*4 >= len(unique)*3
}

func AssessIPRisk(sourceURL string, audits []ImageAudit) (IPRiskAssessment, error) {
	if err := validateIPRiskInput(sourceURL, audits); err != nil {
		return IPRiskAssessment{}, err
	}
	reasons := make([]string, 0)
	score := 0.0
	level := RiskLow
	if strings.Contains(strings.ToLower(strings.TrimSpace(sourceURL)), "1688.com") {
		reasons = append(reasons, "image capability uses a scraped 1688 source")
		score += 0.15
		level = RiskMedium
	}
	for _, audit := range audits {
		object := strings.TrimSpace(audit.PrimaryObject)
		if audit.HasLogo {
			reasons = append(reasons, imageRiskReason("image contains logo or watermark risk", object))
			score += 0.35
			level = maxRiskLevel(level, RiskMedium)
		}
		if audit.HasOverlayText {
			reasons = append(reasons, imageRiskReason("image contains text overlay risk", object))
			score += 0.2
			level = maxRiskLevel(level, RiskMedium)
		}
		if audit.HasPromoBadge {
			reasons = append(reasons, imageRiskReason("image contains promo badge risk", object))
			score += 0.2
			level = maxRiskLevel(level, RiskMedium)
		}
		if containsAnyFold(audit.ImageURL, "brand", "disney", "marvel", "pokemon", "lego", "nike", "adidas", "apple", "dyson", "hello-kitty", "sanrio") {
			reasons = append(reasons, imageRiskReason("image URL indicates branded or protected content risk", object))
			score += 0.5
			level = RiskHigh
		}
	}
	reasons, err := normalizedStrings(reasons, maxIPReasons)
	if err != nil {
		return IPRiskAssessment{}, err
	}
	sort.Strings(reasons)
	if score > 1 {
		score = 1
	}
	return IPRiskAssessment{Level: level, Score: score, Reasons: reasons}, nil
}

const (
	maxIPAudits    = 64
	maxAuditIssues = 64
	maxIPReasons   = 1 + 4*maxIPAudits
)

func validateIPRiskInput(sourceURL string, audits []ImageAudit) error {
	if len(audits) > maxIPAudits || len(sourceURL) > maxImageStringBytes {
		return ErrInputInvalid
	}
	if sourceURL != "" && !isCanonicalHTTPURL(sourceURL) {
		return ErrInputInvalid
	}
	used := len(sourceURL)
	for _, audit := range audits {
		if len(audit.Issues) > maxAuditIssues || len(audit.ImageURL) > maxImageStringBytes || len(audit.PrimaryObject) > maxImageStringBytes {
			return ErrInputInvalid
		}
		if audit.ImageURL != "" && !isCanonicalHTTPURL(audit.ImageURL) {
			return ErrInputInvalid
		}
		used += len(audit.ImageURL) + len(audit.PrimaryObject)
		if used > maxImageInputBytes {
			return ErrInputInvalid
		}
		for _, issue := range audit.Issues {
			if len(issue) > maxImageStringBytes {
				return ErrInputInvalid
			}
			used += len(issue)
			if used > maxImageInputBytes {
				return ErrInputInvalid
			}
		}
	}
	return nil
}

func NormalizeGenerationMetadata(metadata GenerationMetadata) (GenerationMetadata, error) {
	if len(metadata.Values) > maxMetadataValues {
		return GenerationMetadata{}, ErrInputInvalid
	}
	result := GenerationMetadata{
		Capability:      strings.TrimSpace(metadata.Capability),
		ModelFamily:     strings.TrimSpace(metadata.ModelFamily),
		InvocationID:    strings.TrimSpace(metadata.InvocationID),
		PromptReference: strings.TrimSpace(metadata.PromptReference),
		PromptVersion:   strings.TrimSpace(metadata.PromptVersion),
	}
	used := len(result.Capability) + len(result.ModelFamily) + len(result.InvocationID) + len(result.PromptReference) + len(result.PromptVersion)
	for _, value := range []string{result.Capability, result.ModelFamily, result.InvocationID, result.PromptReference, result.PromptVersion} {
		if len(value) > maxImageStringBytes {
			return GenerationMetadata{}, ErrInputInvalid
		}
	}
	if len(metadata.Values) > 0 {
		result.Values = make(map[string]string, len(metadata.Values))
		keys := make([]string, 0, len(metadata.Values))
		for rawKey := range metadata.Values {
			keys = append(keys, rawKey)
		}
		sort.Strings(keys)
		for _, rawKey := range keys {
			rawValue := metadata.Values[rawKey]
			key, value := strings.TrimSpace(rawKey), strings.TrimSpace(rawValue)
			if key == "" || value == "" {
				continue
			}
			if len(key) > maxImageStringBytes || len(value) > maxImageStringBytes {
				return GenerationMetadata{}, ErrInputInvalid
			}
			used += len(key) + len(value)
			if used > maxImageInputBytes {
				return GenerationMetadata{}, ErrInputInvalid
			}
			if _, exists := result.Values[key]; exists {
				return GenerationMetadata{}, ErrInputInvalid
			}
			result.Values[key] = value
		}
		if len(result.Values) == 0 {
			result.Values = nil
		}
	}
	return result, nil
}

func imageRiskReason(message, object string) string {
	if object == "" {
		return message
	}
	return fmt.Sprintf("%s for %s", message, object)
}

func maxRiskLevel(left, right RiskLevel) RiskLevel {
	rank := map[RiskLevel]int{RiskLow: 0, RiskMedium: 1, RiskHigh: 2}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func containsAnyFold(value string, fragments ...string) bool {
	value = strings.ToLower(value)
	for _, fragment := range fragments {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

func minInteger(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func isNilImage(source image.Image) bool {
	value := reflect.ValueOf(source)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
