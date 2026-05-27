package shein

import (
	"fmt"
	"strings"

	common "task-processor/internal/publishing/common"
)

type DisplayAttributeEvidenceItem struct {
	Field           string
	RawValue        string
	NormalizedValue string
	Source          string
	Structured      bool
}

type DisplayAttributeEvidencePool struct {
	Items []DisplayAttributeEvidenceItem
}

func buildDisplayAttributeEvidencePool(pkg *Package) *DisplayAttributeEvidencePool {
	NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	pool := &DisplayAttributeEvidencePool{}
	pool.add("product_title", firstNonEmpty(pkg.SpuName, pkg.ProductNameEn, pkg.ProductNameMulti), "package", false)
	pool.add("category_name", pkg.CategoryName, "package", false)
	if len(pkg.CategoryPath) > 0 {
		pool.add("category_path", strings.Join(pkg.CategoryPath, " > "), "package", false)
	}
	pool.add("description", pkg.Description, "package", false)
	for _, point := range pkg.SellingPoints {
		pool.add("selling_point", point, "package", false)
	}
	for _, attr := range pkg.ProductAttributes {
		pool.add(attr.Name, attr.Value, "product_attributes", false)
		pool.addStructuredSegments(attr.Name, attr.Value, "product_attributes")
	}
	for name, value := range pkg.Attributes {
		pool.add(name, value, "attributes", false)
		pool.addStructuredSegments(name, value, "attributes")
	}
	if pkg.DraftPayload != nil && len(pkg.DraftPayload.SKCList) > 0 && len(pkg.DraftPayload.SKCList[0].SKUList) > 0 {
		sku := pkg.DraftPayload.SKCList[0].SKUList[0]
		pool.add("supplier_sku", sku.SupplierSKU, "request_draft", false)
		pool.add("length_cm", sku.Length, "request_draft", true)
		pool.add("width_cm", sku.Width, "request_draft", true)
		pool.add("height_cm", sku.Height, "request_draft", true)
		pool.add("variant_weight", formatFloatEvidence(sku.Weight), "request_draft", true)
		pool.add("variant_weight_unit", sku.WeightUnit, "request_draft", true)
	}
	if len(pool.Items) == 0 {
		return nil
	}
	return pool
}

func newDisplayAttributeEvidencePoolFromInputs(inputs []common.Attribute) *DisplayAttributeEvidencePool {
	if len(inputs) == 0 {
		return nil
	}
	pool := &DisplayAttributeEvidencePool{}
	for _, input := range inputs {
		pool.add(input.Name, input.Value, "inputs", false)
	}
	if len(pool.Items) == 0 {
		return nil
	}
	return pool
}

func (p *DisplayAttributeEvidencePool) add(field string, raw string, source string, structured bool) {
	if p == nil {
		return
	}
	field = strings.TrimSpace(field)
	raw = strings.TrimSpace(raw)
	if field == "" || raw == "" {
		return
	}
	p.Items = append(p.Items, DisplayAttributeEvidenceItem{
		Field:           field,
		RawValue:        raw,
		NormalizedValue: normalizeText(raw),
		Source:          source,
		Structured:      structured,
	})
}

func (p *DisplayAttributeEvidencePool) addStructuredSegments(field string, raw string, source string) {
	field = strings.TrimSpace(field)
	raw = strings.TrimSpace(raw)
	if p == nil || field == "" || raw == "" {
		return
	}
	switch normalizeText(field) {
	case "product size", "packaging specification", "product performance":
	default:
		return
	}
	for _, segment := range splitEvidenceSegments(raw) {
		if segment == "" || !containsStructuredEvidence(segment) {
			continue
		}
		p.add(field+"_segment", segment, source, true)
	}
}

func (p *DisplayAttributeEvidencePool) HasField(field string) bool {
	key := normalizeText(field)
	for _, item := range p.Items {
		if normalizeText(item.Field) == key {
			return true
		}
	}
	return false
}

func (p *DisplayAttributeEvidencePool) FieldNames() []string {
	if p == nil {
		return nil
	}
	names := make([]string, 0, len(p.Items))
	seen := make(map[string]struct{}, len(p.Items))
	for _, item := range p.Items {
		key := normalizeText(item.Field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, item.Field)
	}
	return names
}

func (p *DisplayAttributeEvidencePool) StructuredItems() []DisplayAttributeEvidenceItem {
	if p == nil {
		return nil
	}
	items := make([]DisplayAttributeEvidenceItem, 0)
	for _, item := range p.Items {
		if item.Structured {
			items = append(items, item)
		}
	}
	return items
}

func (p *DisplayAttributeEvidencePool) FirstValue(fields ...string) string {
	if p == nil {
		return ""
	}
	for _, field := range fields {
		key := normalizeText(field)
		for _, item := range p.Items {
			if normalizeText(item.Field) == key && strings.TrimSpace(item.RawValue) != "" {
				return strings.TrimSpace(item.RawValue)
			}
		}
	}
	return ""
}

func (p *DisplayAttributeEvidencePool) Values(field string) []string {
	if p == nil {
		return nil
	}
	key := normalizeText(field)
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range p.Items {
		if normalizeText(item.Field) != key {
			continue
		}
		value := strings.TrimSpace(item.RawValue)
		if value == "" {
			continue
		}
		normalized := normalizeText(value)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		values = append(values, value)
	}
	return values
}

func (p *DisplayAttributeEvidencePool) AttributeInputs() []common.Attribute {
	if p == nil {
		return nil
	}
	inputs := make([]common.Attribute, 0, len(p.Items)+8)
	inputs = append(inputs, buildDerivedAttributeInputsFromEvidence(p)...)
	for _, item := range p.Items {
		if strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.RawValue) == "" {
			continue
		}
		inputs = append(inputs, common.Attribute{Name: item.Field, Value: item.RawValue})
	}
	if len(inputs) == 0 {
		return nil
	}
	return dedupeAttributeInputs(inputs)
}

func (p *DisplayAttributeEvidencePool) ResolutionInputs() []common.Attribute {
	if p == nil {
		return nil
	}
	inputs := make([]common.Attribute, 0, len(p.Items))
	appendBySource := func(source string) {
		for _, item := range p.Items {
			if item.Source != source {
				continue
			}
			if strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.RawValue) == "" {
				continue
			}
			inputs = append(inputs, common.Attribute{Name: item.Field, Value: item.RawValue})
		}
	}
	appendBySource("product_attributes")
	appendBySource("attributes")
	appendBySource("request_draft")
	appendBySource("package")
	appendBySource("inputs")
	if len(inputs) == 0 {
		return p.AttributeInputs()
	}
	return dedupeAttributeInputs(inputs)
}

func buildDerivedAttributeInputsFromEvidence(pool *DisplayAttributeEvidencePool) []common.Attribute {
	if pool == nil {
		return nil
	}
	result := make([]common.Attribute, 0, 24)
	appendValue := func(name string, value string) {
		value = strings.TrimSpace(value)
		if strings.TrimSpace(name) == "" || value == "" {
			return
		}
		result = append(result, common.Attribute{Name: name, Value: value})
	}

	appendValue("Product Title", pool.FirstValue("product_title", "spu_name"))
	appendValue("Category Name", pool.FirstValue("category_name"))
	appendValue("Category Path", pool.FirstValue("category_path"))
	appendValue("Description", pool.FirstValue("description"))
	appendValue("Material", pool.FirstValue("material", "material_description"))
	appendValue("Material Description", pool.FirstValue("material_description", "material"))
	appendValue("Production Process", pool.FirstValue("production_process"))
	appendValue("Product Performance", pool.FirstValue("product_performance"))
	appendValue("Applicable Scenarios", pool.FirstValue("applicable_scenarios"))
	appendValue("Special Description", pool.FirstValue("special_description"))
	appendValue("Product Size", pool.FirstValue("product_size", "size"))
	appendValue("Packaging Specification", pool.FirstValue("packaging_specification"))
	appendValue("Product SKU", pool.FirstValue("product_sku"))
	appendValue("Variant SKU", pool.FirstValue("variant_sku", "source_sds_sku", "supplier_sku"))
	appendValue("Product Model", firstNonEmpty(
		pool.FirstValue("product_model"),
		pool.FirstValue("variant_sku", "source_sds_sku", "supplier_sku"),
		pool.FirstValue("product_sku"),
	))
	appendValue("Variant Size", pool.FirstValue("variant_size", "size"))
	appendValue("Variant Color", pool.FirstValue("variant_color", "color"))
	appendValue("Length (cm)", pool.FirstValue("length_cm"))
	appendValue("Width (cm)", pool.FirstValue("width_cm"))
	appendValue("Height (cm)", pool.FirstValue("height_cm"))
	appendValue("Weight", pool.FirstValue("variant_weight"))
	if unit := strings.ToLower(strings.TrimSpace(pool.FirstValue("variant_weight_unit"))); unit != "" {
		appendValue("Weight ("+unit+")", pool.FirstValue("variant_weight"))
	}
	for _, point := range pool.Values("selling_point") {
		appendValue("Selling Point", point)
	}
	return dedupeAttributeInputs(result)
}

func splitEvidenceSegments(value string) []string {
	replacer := strings.NewReplacer("；", "\n", ";", "\n", "，", "\n", ",", "\n", "|", "\n")
	parts := strings.Split(replacer.Replace(value), "\n")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func containsStructuredEvidence(value string) bool {
	hasDigit := false
	for _, r := range value {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return false
	}
	normalized := normalizeText(value)
	return strings.Contains(normalized, "cm") ||
		strings.Contains(normalized, "inch") ||
		strings.Contains(normalized, "kg") ||
		strings.Contains(normalized, "mm") ||
		strings.Contains(normalized, "x")
}

func formatFloatEvidence(value float64) string {
	if value == 0 {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%g", value))
}
