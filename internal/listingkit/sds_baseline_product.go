package listingkit

import (
	"strings"

	"task-processor/internal/product/catalog/canonical"
)

func newSDSBaselineCanonicalProduct(text string, sds *SDSSyncOptions) *canonical.Product {
	if sds == nil {
		return nil
	}
	title := firstNonEmptyString(sds.ProductName, text, sds.ProductEnglishName)
	description := firstNonEmptyString(sds.ProductPerformance, sds.MaterialDescription, text, title)
	categoryPath := make([]string, 0, len(sds.CategoryPath))
	for _, category := range sds.CategoryPath {
		if category = strings.TrimSpace(category); category != "" {
			categoryPath = append(categoryPath, category)
		}
	}
	return &canonical.Product{
		Title:        title,
		Description:  description,
		CategoryPath: categoryPath,
	}
}
