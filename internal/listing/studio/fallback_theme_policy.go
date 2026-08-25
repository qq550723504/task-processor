package studio

import "strings"

// BuildFallbackDesignThemes repeats the normalized source prompt when prompt
// diversification cannot produce enough sibling themes.
func BuildFallbackDesignThemes(prompt string, count int) []string {
	if count <= 0 {
		return nil
	}
	theme := strings.TrimSpace(prompt)
	themes := make([]string, count)
	for idx := range themes {
		themes[idx] = theme
	}
	return themes
}
