package shein

import (
	"encoding/json"
	"regexp"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

const defaultSizeSaleAttributeID = 87

var (
	cmValuePattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*cm\b`)

	apparelSizeAttributeIDs = map[string]int{
		"肩宽": 10,
		"胸围": 15,
		"衣长": 20,
		"袖长": 29,
	}
)

type sdsSizeCell struct {
	Content string `json:"content"`
}

type sizeSaleAttributeRef struct {
	attributeID      int
	attributeValueID int
}

func applyProductSizeAttributes(pkg *Package, productSize string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	attrs := buildSizeAttributesFromProductSize(productSize, pkg)
	pkg.DraftPayload.SizeAttributeList = attrs
	return len(attrs) > 0
}

func buildSizeAttributesFromProductSize(productSize string, pkg *Package) []sheinproduct.SizeAttribute {
	table := parseSDSProductSizeTable(productSize)
	if len(table) < 2 || len(table[0]) < 2 {
		return nil
	}
	sizeValues := collectSizeSaleAttributeRefs(pkg)
	if len(sizeValues) == 0 {
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
		ref, ok := sizeValues[normalizeText(row[0])]
		if !ok || ref.attributeID <= 0 || ref.attributeValueID <= 0 {
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
				RelateSaleAttributeID:      ref.attributeID,
				RelateSaleAttributeValueID: ref.attributeValueID,
			})
		}
	}
	return result
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

func collectSizeSaleAttributeRefs(pkg *Package) map[string]sizeSaleAttributeRef {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	sizeAttributeID := defaultSizeSaleAttributeID
	sizeSourceDimension := "size"
	if pkg.SaleAttributeResolution != nil {
		if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 && isSizeSourceDimension(pkg.SaleAttributeResolution.SecondarySourceDimension) {
			sizeAttributeID = pkg.SaleAttributeResolution.SecondaryAttributeID
		}
		if strings.TrimSpace(pkg.SaleAttributeResolution.SecondarySourceDimension) != "" {
			sizeSourceDimension = pkg.SaleAttributeResolution.SecondarySourceDimension
		}
	}
	result := map[string]sizeSaleAttributeRef{}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID != sizeAttributeID || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					continue
				}
				sizeValue := firstNonEmpty(attr.Value, lookupAttributeValue(sku.Attributes, sizeSourceDimension), lookupAttributeValue(sku.Attributes, "Size"), lookupAttributeValue(sku.Attributes, "尺码"))
				if strings.TrimSpace(sizeValue) == "" {
					continue
				}
				result[normalizeText(sizeValue)] = sizeSaleAttributeRef{
					attributeID:      attr.AttributeID,
					attributeValueID: *attr.AttributeValueID,
				}
			}
		}
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

func isSizeSourceDimension(value string) bool {
	switch normalizeText(value) {
	case "size", "尺码", "尺寸", "规格":
		return true
	default:
		return false
	}
}
