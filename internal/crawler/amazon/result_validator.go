package amazon

import (
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/model"
)

// ProductQualityError marks a crawl as structurally successful but unusable for downstream business flows.
type ProductQualityError struct {
	Reasons []string
}

func (e *ProductQualityError) Error() string {
	if len(e.Reasons) == 0 {
		return "product quality validation failed"
	}
	return fmt.Sprintf("product quality validation failed: %s", strings.Join(e.Reasons, "; "))
}

func isProductQualityError(err error) bool {
	var target *ProductQualityError
	return errors.As(err, &target)
}

type ProductResultValidator struct{}

func NewProductResultValidator() *ProductResultValidator {
	return &ProductResultValidator{}
}

func (v *ProductResultValidator) Validate(product *model.Product) error {
	if product == nil {
		return &ProductQualityError{Reasons: []string{"product is nil"}}
	}

	var reasons []string

	if strings.TrimSpace(product.Asin) == "" {
		reasons = append(reasons, "asin is empty")
	}
	if strings.TrimSpace(product.Title) == "" {
		reasons = append(reasons, "title is empty")
	}
	if !hasPrimaryImage(product) {
		reasons = append(reasons, "primary image is missing")
	}
	if requiresPrice(product) && !hasUsablePrice(product) {
		reasons = append(reasons, "price is missing")
	}
	if hasBrokenVariationPayload(product) {
		reasons = append(reasons, "variation payload is incomplete")
	}

	if len(reasons) > 0 {
		return &ProductQualityError{Reasons: reasons}
	}

	return nil
}

func hasPrimaryImage(product *model.Product) bool {
	if product == nil {
		return false
	}
	if strings.TrimSpace(product.ImageURL) != "" {
		return true
	}
	for _, image := range product.Images {
		if strings.TrimSpace(image) != "" {
			return true
		}
	}
	return false
}

func requiresPrice(product *model.Product) bool {
	if product == nil {
		return false
	}
	if !product.IsAvailable {
		return false
	}
	return strings.TrimSpace(product.Availability) != "Currently unavailable"
}

func hasUsablePrice(product *model.Product) bool {
	if product == nil {
		return false
	}
	if product.FinalPrice > 0 {
		return true
	}
	return product.BuyboxPrices != nil && product.BuyboxPrices.FinalPrice > 0
}

func hasBrokenVariationPayload(product *model.Product) bool {
	if product == nil {
		return false
	}
	if len(product.Variations) == 0 {
		return false
	}
	if len(product.VariationsValues) == 0 {
		return true
	}
	return false
}
