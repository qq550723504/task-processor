package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

type categorySemanticVerifier interface {
	ValidateProductCategory(ctx context.Context, canonical *canonical.Product, pkg *Package, categoryPath []string) *CategorySemanticValidation
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

func (v *aiCategorySemanticVerifier) ValidateProductCategory(ctx context.Context, canonical *canonical.Product, pkg *Package, categoryPath []string) *CategorySemanticValidation {
	if v == nil || v.client == nil || len(categoryPath) == 0 {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
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

func buildCategorySemanticValidationPrompt(canonical *canonical.Product, pkg *Package, categoryPath []string) string {
	return renderSheinDisplayAttributePrompt(prompt.KSheinCategorySelectorSemanticValidation, `You validate whether a SHEIN category path matches the actual product semantics.
Focus on what the product physically is, not on noisy or misleading title words.
Return JSON only with keys verdict and reason.
verdict must be one of: compatible, incompatible, uncertain.

Candidate SHEIN category path:
- {{.CategoryPath}}

Product summary:
{{.ProductSummary}}`, map[string]any{
		"CategoryPath":   strings.Join(categoryPath, " > "),
		"ProductSummary": buildCategorySemanticProductSummary(canonical, pkg),
	})
}

func buildCategorySemanticProductSummary(canonical *canonical.Product, pkg *Package) string {
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
