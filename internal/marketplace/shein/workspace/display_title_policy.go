package workspace

import "strings"

// DisplayTitleInput contains the title fields needed by the SHEIN workspace preview.
type DisplayTitleInput struct {
	ProductNameEn    string
	ProductNameMulti string
	SpuName          string
	TitleSource      string
}

// ResolveDisplayTitle chooses the safe title shown by the SHEIN workspace preview.
func ResolveDisplayTitle(input DisplayTitleInput) string {
	title := firstNonEmpty(input.ProductNameEn, input.ProductNameMulti)
	if strings.TrimSpace(title) != "" {
		return title
	}
	source := strings.TrimSpace(input.TitleSource)
	if source == "unresolved_prompt_title" || source == "structured_fallback" {
		return ""
	}
	return strings.TrimSpace(input.SpuName)
}
