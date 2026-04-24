package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/productenrich"
)

type categorySemanticVerifier interface {
	ValidateProductCategory(canonical *productenrich.CanonicalProduct, pkg *Package, categoryPath []string) *CategorySemanticValidation
}

type aiCategorySemanticVerifier struct {
	client openaiclient.ChatCompleter
}

func newAICategorySemanticVerifier(client openaiclient.ChatCompleter) categorySemanticVerifier {
	if client == nil {
		return nil
	}
	return &aiCategorySemanticVerifier{client: client}
}

func (v *aiCategorySemanticVerifier) ValidateProductCategory(canonical *productenrich.CanonicalProduct, pkg *Package, categoryPath []string) *CategorySemanticValidation {
	if v == nil || v.client == nil || len(categoryPath) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	raw, err := v.client.Generate(ctx, buildCategorySemanticValidationPrompt(canonical, pkg, categoryPath))
	if err != nil {
		return nil
	}
	raw = jsonx.CleanLLMResponse(raw)
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var payload struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	verdict := strings.ToLower(strings.TrimSpace(payload.Verdict))
	switch verdict {
	case "compatible", "incompatible", "uncertain":
	default:
		return nil
	}

	return &CategorySemanticValidation{
		Source:       "ai_semantic_validation",
		ComparedPath: append([]string(nil), categoryPath...),
		Verdict:      verdict,
		Reason:       strings.TrimSpace(payload.Reason),
	}
}

func buildCategorySemanticValidationPrompt(canonical *productenrich.CanonicalProduct, pkg *Package, categoryPath []string) string {
	var builder strings.Builder
	builder.WriteString("You validate whether a SHEIN category path matches the actual product semantics.\n")
	builder.WriteString("Focus on what the product physically is, not on noisy or misleading title words.\n")
	builder.WriteString("Return JSON only with keys verdict and reason.\n")
	builder.WriteString("verdict must be one of: compatible, incompatible, uncertain.\n\n")
	builder.WriteString("Candidate SHEIN category path:\n")
	builder.WriteString("- ")
	builder.WriteString(strings.Join(categoryPath, " > "))
	builder.WriteString("\n\n")
	builder.WriteString("Product summary:\n")
	builder.WriteString(buildCategorySemanticProductSummary(canonical, pkg))
	builder.WriteString("\n\nExamples:\n")
	builder.WriteString("- bench cushion / chair cushion / outdoor seat pad should be home furnishing, not apparel.\n")
	builder.WriteString("- tumbler / water bottle should be drinkware, not footwear.\n")
	builder.WriteString("- costume / dress / pajama should be apparel, not furniture.\n")
	return builder.String()
}

func buildCategorySemanticProductSummary(canonical *productenrich.CanonicalProduct, pkg *Package) string {
	lines := make([]string, 0, 8)
	appendLine := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", label, value))
	}

	if canonical != nil {
		appendLine("title", canonical.Title)
		if len(canonical.CategoryPath) > 0 {
			appendLine("source_category_path", strings.Join(canonical.CategoryPath, " > "))
		}
	}

	summaryKeys := []string{"产品类别", "category", "品类", "材质", "填充物", "空间", "风格", "尺寸", "用途", "颜色"}
	for _, key := range summaryKeys {
		if canonical != nil {
			if attr, ok := canonical.Attributes[key]; ok {
				appendLine("attribute_"+key, attr.Value)
			}
		}
		if pkg != nil {
			if value := strings.TrimSpace(pkg.Attributes[key]); value != "" {
				appendLine("package_"+key, value)
			}
		}
	}

	if canonical != nil && len(canonical.VariantDimensions) > 0 {
		for _, dim := range canonical.VariantDimensions {
			if strings.TrimSpace(dim.Name) == "" || len(dim.Values) == 0 {
				continue
			}
			appendLine("variant_"+dim.Name, strings.Join(dim.Values, " | "))
		}
	}

	// Keep the semantic summary anchored to structured evidence.
	// Free-form descriptions are often noisy in this pipeline and can
	// incorrectly drag the verifier toward unrelated product types.
	if len(lines) < 3 && canonical != nil {
		appendLine("description", canonical.Description)
	}

	if len(lines) == 0 {
		return "- summary: unavailable"
	}
	return strings.Join(dedupeStrings(lines), "\n")
}
