package attribute

import (
	"strconv"
	"strings"

	sheinapi "task-processor/internal/shein/api/attribute"
)

type platformValueDomain string

const (
	platformValueDomainGeneric            platformValueDomain = "generic"
	platformValueDomainShoeMetricSize     platformValueDomain = "shoe_metric_size"
	platformValueDomainApparelAlphaSize   platformValueDomain = "apparel_alpha_size"
	platformValueDomainApparelNumericSize platformValueDomain = "apparel_numeric_size"
	platformValueDomainNumericSizeLike    platformValueDomain = "numeric_size_like"
)

func findTemplateAttributeInfo(attrID int, templates *sheinapi.AttributeTemplateInfo) *sheinapi.AttributeInfo {
	if templates == nil {
		return nil
	}
	for _, data := range templates.Data {
		for i := range data.AttributeInfos {
			if data.AttributeInfos[i].AttributeID == attrID {
				return &data.AttributeInfos[i]
			}
		}
	}
	return nil
}

func detectPlatformValueDomain(attrInfo *sheinapi.AttributeInfo) platformValueDomain {
	if attrInfo == nil || !isSizeLikeAttribute(attrInfo) {
		return platformValueDomainGeneric
	}

	profile := buildAttributeValueProfile(attrInfo.AttributeValueInfoList)
	switch {
	case profile.looksLikeMetricShoeSize():
		return platformValueDomainShoeMetricSize
	case profile.looksLikeApparelAlphaSize():
		return platformValueDomainApparelAlphaSize
	case profile.looksNumericSizeLike():
		return platformValueDomainApparelNumericSize
	default:
		return platformValueDomainGeneric
	}
}

func isSizeLikeAttribute(attrInfo *sheinapi.AttributeInfo) bool {
	if attrInfo == nil {
		return false
	}
	if attrInfo.AttributeID == 87 {
		return true
	}

	name := strings.ToLower(strings.TrimSpace(attrInfo.AttributeName + " " + attrInfo.AttributeNameEn))
	return strings.Contains(name, "size") ||
		strings.Contains(name, "尺码") ||
		strings.Contains(name, "尺寸")
}

type attributeValueProfile struct {
	totalCount     int
	numericCount   int
	metricShoeLike int
	alphaSizeLike  int
}

func buildAttributeValueProfile(values []sheinapi.AttributeValue) attributeValueProfile {
	seen := make(map[string]struct{}, len(values))
	profile := attributeValueProfile{}

	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		profile.totalCount++

		numeric, err := strconv.ParseFloat(value, 64)
		if err == nil {
			profile.numericCount++
			if numeric >= 180 && numeric <= 320 {
				profile.metricShoeLike++
			}
		}
		if _, ok := normalizeAlphaSizeLabel(value); ok {
			profile.alphaSizeLike++
		}
	}

	for _, value := range values {
		appendValue(value.AttributeValue)
		appendValue(value.AttributeValueEn)
	}

	return profile
}

func (p attributeValueProfile) looksLikeMetricShoeSize() bool {
	if p.totalCount == 0 || p.numericCount == 0 || p.metricShoeLike == 0 {
		return false
	}
	return p.metricShoeLike*2 >= p.numericCount && p.numericCount*2 >= p.totalCount
}

func (p attributeValueProfile) looksNumericSizeLike() bool {
	if p.totalCount == 0 || p.numericCount == 0 {
		return false
	}
	return p.numericCount*2 >= p.totalCount
}

func (p attributeValueProfile) looksLikeApparelAlphaSize() bool {
	if p.totalCount == 0 || p.alphaSizeLike == 0 {
		return false
	}
	return p.alphaSizeLike*2 >= p.totalCount
}
