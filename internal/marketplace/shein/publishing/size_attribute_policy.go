package publishing

import (
	"encoding/json"
	"regexp"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

var cmValuePattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*cm\b`)

var apparelSizeAttributeIDs = map[string]int{
	"肩宽": 10,
	"胸围": 15,
	"衣长": 20,
	"袖长": 29,
}

type sdsSizeCell struct {
	Content string `json:"content"`
}

// SizeSaleAttributeRef links a source size label to the resolved SHEIN sale attribute value.
type SizeSaleAttributeRef struct {
	SizeValue        string
	AttributeID      int
	AttributeValueID int
}

// BuildSizeAttributesFromProductSize converts structured SDS product_size data into SHEIN size_attribute_list rows.
func BuildSizeAttributesFromProductSize(productSize string, sizeValues []SizeSaleAttributeRef) []sheinproduct.SizeAttribute {
	table := parseSDSProductSizeTable(productSize)
	if len(table) < 2 || len(table[0]) < 2 {
		return nil
	}
	sizeValueRefs := indexSizeSaleAttributeRefs(sizeValues)
	if len(sizeValueRefs) == 0 {
		return nil
	}
	headerAttributeIDs := make([]int, len(table[0]))
	for index := 1; index < len(table[0]); index++ {
		headerAttributeIDs[index] = sdsApparelSizeAttributeID(table[0][index])
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

func sdsApparelSizeAttributeID(header string) int {
	header = normalizeSDSSizeHeader(header)
	return apparelSizeAttributeIDs[header]
}

func normalizeSDSSizeHeader(header string) string {
	header = strings.TrimSpace(header)
	if index := strings.Index(header, "("); index >= 0 {
		header = header[:index]
	}
	if index := strings.Index(header, "（"); index >= 0 {
		header = header[:index]
	}
	return strings.TrimSpace(header)
}

func extractCentimeterValue(value string) string {
	matches := cmValuePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeSizeText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
