package shein

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

var ErrEnglishDescriptionGenerationFailed = errors.New("English product description generation failed")

func resolveListingDescription(ctx context.Context, product *canonical.Product, title string, ai TextGenerator) (string, error) {
	if existing := firstEnglishCandidate(canonicalDescription(product)); existing != "" {
		return existing, nil
	}
	if ai == nil {
		return "", ErrEnglishDescriptionGenerationFailed
	}
	prompt := fmt.Sprintf("Write one factual English e-commerce product description. Use only supported facts. Title: %s. Category: %s. Source description: %s. Return only the description.", cleanListingText(title), strings.Join(product.CategoryPath, " > "), cleanListingText(canonicalDescription(product)))
	value, err := ai.Generate(ctx, prompt)
	value = cleanListingText(value)
	if err != nil || value == "" || containsCJK(value) {
		return "", ErrEnglishDescriptionGenerationFailed
	}
	return value, nil
}
