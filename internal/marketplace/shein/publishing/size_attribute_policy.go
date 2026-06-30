package publishing

import (
	"encoding/json"
	"regexp"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

var cmValuePattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*cm\b`)
var leadingNumericValuePattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\b`)

var apparelSizeAttributeIDs = map[string]int{
	"肩宽": 10,
	"胸围": 15,
	"长度": 20,
	"衣长": 20,
	"袖长": 29,
}

var sizeHeaderAliases = map[string]string{
	"衣长": "长度",
	"脚长": "可穿脚长",
	"足长": "可穿脚长",
	"脚围": "掌围",
}

type sdsSizeCell struct {
	Content string `json:"content"`
}

type SizeChartTemplateAttribute struct {
	AttributeID     int
	AttributeName   string
	AttributeNameEn string
	SourceSystemIDs []int
	SortOrder       int
}

// SizeSaleAttributeRef links a source size label to the resolved SHEIN sale attribute value.
type SizeSaleAttributeRef struct {
	SizeValue        string
	AttributeID      int
	AttributeValueID int
}

// BuildSizeAttributesFromProductSize converts structured SDS product_size data into SHEIN size_attribute_list rows.
func BuildSizeAttributesFromProductSize(productSize string, sizeValues []SizeSaleAttributeRef) []sheinproduct.SizeAttribute {
	return BuildSizeAttributesFromProductSizeWithTemplates(productSize, sizeValues, nil)
}

func BuildSizeAttributesFromProductSizeWithTemplates(productSize string, sizeValues []SizeSaleAttributeRef, templateAttrs []SizeChartTemplateAttribute) []sheinproduct.SizeAttribute {
	return BuildSizeAttributesFromProductSizeWithHeaderAttributeIDs(productSize, sizeValues, templateAttrs, nil)
}

func BuildSizeAttributesFromProductSizeWithHeaderAttributeIDs(productSize string, sizeValues []SizeSaleAttributeRef, templateAttrs []SizeChartTemplateAttribute, headerAttributeIDsByName map[string]int) []sheinproduct.SizeAttribute {
	table := parseSDSProductSizeTable(productSize)
	if len(table) < 2 || len(table[0]) < 2 {
		return nil
	}
	sizeValueRefs := indexSizeSaleAttributeRefs(sizeValues)
	if len(sizeValueRefs) == 0 {
		return nil
	}
	headerAttributeIDs := make([]int, len(table[0]))
	templateAttributeIDs := indexSizeChartTemplateAttributes(templateAttrs)
	validTemplateAttributeIDs := indexSizeChartTemplateAttributeIDs(templateAttrs)
	headerAttributeIDOverrides := indexSizeHeaderAttributeIDOverrides(headerAttributeIDsByName)
	for index := 1; index < len(table[0]); index++ {
		headerAttributeIDs[index] = sdsSizeAttributeID(table[0][index], templateAttributeIDs, headerAttributeIDOverrides, validTemplateAttributeIDs)
	}

	result := make([]sheinproduct.SizeAttribute, 0, (len(table)-1)*(len(table[0])-1))
	for rowIndex := 1; rowIndex < len(table); rowIndex++ {
		row := table[rowIndex]
		if len(row) == 0 {
			continue
		}
		ref, ok := sizeValueRefs[normalizeSizeText(row[0])]
		if !ok || ref.AttributeID <= 0 || ref.AttributeValueID <= 0 {
			continue
		}
		for columnIndex := 1; columnIndex < len(row) && columnIndex < len(headerAttributeIDs); columnIndex++ {
			attributeID := headerAttributeIDs[columnIndex]
			if attributeID <= 0 {
				continue
			}
			value := extractCentimeterValue(row[columnIndex])
			if value == "" {
				continue
			}
			result = append(result, sheinproduct.SizeAttribute{
				AttributeID:                attributeID,
				AttributeExtraValue:        value,
				RelateSaleAttributeID:      ref.AttributeID,
				RelateSaleAttributeValueID: ref.AttributeValueID,
			})
		}
	}
	return result
}

func UnmappedSDSSizeHeaders(productSize string, templateAttrs []SizeChartTemplateAttribute) []string {
	table := parseSDSProductSizeTable(productSize)
	if len(table) == 0 || len(table[0]) < 2 {
		return nil
	}
	templateAttributeIDs := indexSizeChartTemplateAttributes(templateAttrs)
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for index := 1; index < len(table[0]); index++ {
		header := strings.TrimSpace(table[0][index])
		if header == "" {
			continue
		}
		if sdsSizeAttributeID(header, templateAttributeIDs, nil, nil) > 0 {
			continue
		}
		key := normalizeSDSSizeHeader(header)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, header)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func IsSizeChartTemplateAttribute(attr sheinattribute.AttributeInfo) bool {
	if attr.AttributeType != 2 ||
		attr.AttributeMode != 0 ||
		attr.DataDimension != 2 ||
		len(attr.AttributeRemarkList) != 0 {
		return false
	}
	if intSlicesEqual(attr.SourceSystemIDList, []int{1, 2, 6, 7}) {
		return true
	}
	return isKnownGarmentSizeChartAttributeName(attr.AttributeName) ||
		isKnownGarmentSizeChartAttributeName(attr.AttributeNameEn)
}

func isKnownGarmentSizeChartAttributeName(value string) bool {
	name := normalizeSDSSizeHeader(value)
	switch name {
	case "长度", "胸围", "腰围", "袖长", "肩宽", "臀围", "下摆围", "可穿脚长", "掌围":
		return true
	case "length", "bust", "waist", "sleeve length", "shoulder", "hips", "hip size", "hem width", "foot length", "ball girth":
		return true
	default:
		return false
	}
}

// IsSizeSourceDimension reports whether a resolved source dimension represents product size.
func IsSizeSourceDimension(value string) bool {
	switch normalizeSizeText(value) {
	case "size", "尺码", "尺寸", "规格":
		return true
	default:
		return false
	}
}

func parseSDSProductSizeTable(productSize string) [][]string {
	productSize = strings.TrimSpace(productSize)
	if productSize == "" {
		return nil
	}
	var raw [][]sdsSizeCell
	if err := json.Unmarshal([]byte(productSize), &raw); err != nil {
		return nil
	}
	result := make([][]string, 0, len(raw))
	for _, row := range raw {
		cells := make([]string, 0, len(row))
		for _, cell := range row {
			cells = append(cells, strings.TrimSpace(cell.Content))
		}
		result = append(result, cells)
	}
	return result
}

func indexSizeSaleAttributeRefs(values []SizeSaleAttributeRef) map[string]SizeSaleAttributeRef {
	result := map[string]SizeSaleAttributeRef{}
	for _, value := range values {
		if strings.TrimSpace(value.SizeValue) == "" || value.AttributeID <= 0 || value.AttributeValueID <= 0 {
			continue
		}
		result[normalizeSizeText(value.SizeValue)] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func indexSizeChartTemplateAttributes(attrs []SizeChartTemplateAttribute) map[string]int {
	if len(attrs) == 0 {
		return nil
	}
	result := map[string]int{}
	for _, attr := range attrs {
		if attr.AttributeID <= 0 {
			continue
		}
		for _, name := range []string{attr.AttributeName, attr.AttributeNameEn} {
			normalized := normalizeSDSSizeHeader(name)
			if normalized == "" {
				continue
			}
			result[normalized] = attr.AttributeID
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func indexSizeChartTemplateAttributeIDs(attrs []SizeChartTemplateAttribute) map[int]struct{} {
	if len(attrs) == 0 {
		return nil
	}
	result := map[int]struct{}{}
	for _, attr := range attrs {
		if attr.AttributeID > 0 {
			result[attr.AttributeID] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func indexSizeHeaderAttributeIDOverrides(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}
	result := map[string]int{}
	for header, attributeID := range values {
		normalized := normalizeSDSSizeHeader(header)
		if normalized == "" || attributeID <= 0 {
			continue
		}
		result[normalized] = attributeID
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func sdsSizeAttributeID(header string, templateAttributeIDs map[string]int, headerAttributeIDOverrides map[string]int, validTemplateAttributeIDs map[int]struct{}) int {
	header = normalizeSDSSizeHeader(header)
	if attributeID := templateAttributeIDs[header]; attributeID > 0 {
		return attributeID
	}
	if attributeID := headerAttributeIDOverrides[header]; attributeID > 0 && isValidSizeHeaderAttributeID(attributeID, validTemplateAttributeIDs) {
		return attributeID
	}
	if len(templateAttributeIDs) > 0 {
		return 0
	}
	return apparelSizeAttributeIDs[header]
}

func isValidSizeHeaderAttributeID(attributeID int, validTemplateAttributeIDs map[int]struct{}) bool {
	if attributeID <= 0 {
		return false
	}
	if len(validTemplateAttributeIDs) == 0 {
		return true
	}
	_, ok := validTemplateAttributeIDs[attributeID]
	return ok
}

func normalizeSDSSizeHeader(header string) string {
	header = strings.TrimSpace(header)
	if index := strings.Index(header, "("); index >= 0 {
		header = header[:index]
	}
	if index := strings.Index(header, "（"); index >= 0 {
		header = header[:index]
	}
	header = strings.ToLower(strings.TrimSpace(header))
	if alias := sizeHeaderAliases[header]; alias != "" {
		return alias
	}
	return header
}

func extractCentimeterValue(value string) string {
	value = strings.TrimSpace(value)
	matches := cmValuePattern.FindStringSubmatch(value)
	if len(matches) < 2 {
		matches = leadingNumericValuePattern.FindStringSubmatch(value)
		if len(matches) < 2 {
			return ""
		}
	}
	return strings.TrimSpace(matches[1])
}

func normalizeSizeText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func intSlicesEqual(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
