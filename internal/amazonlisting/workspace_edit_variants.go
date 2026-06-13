package amazonlisting

import (
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func applyVariantDraftEdit(draft *AmazonListingDraft, index int, subfield string, edit DraftFieldEdit) error {
	ensureDraftVariant(draft, index)
	variant := &draft.Variants[index]
	switch {
	case subfield == "sku":
		variant.SKU = strings.TrimSpace(edit.StringValue)
	case subfield == "barcode":
		variant.Barcode = strings.TrimSpace(edit.StringValue)
	case subfield == "inventory":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].inventory requires number_value", index)
		}
		variant.Inventory = int(*edit.NumberValue)
	case subfield == "is_default":
		value, err := parseBooleanEdit(edit, fmt.Sprintf("variants[%d].is_default", index))
		if err != nil {
			return err
		}
		variant.IsDefault = value
	case subfield == "main_image":
		variant.MainImage = strings.TrimSpace(edit.StringValue)
	case subfield == "price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].price.amount requires number_value", index)
		}
		ensureDraftVariantPrice(variant)
		variant.Price.Amount = *edit.NumberValue
	case subfield == "price.currency":
		ensureDraftVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case subfield == "cost_price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].cost_price.amount requires number_value", index)
		}
		ensureDraftVariantCostPrice(variant)
		variant.CostPrice.Amount = *edit.NumberValue
	case subfield == "cost_price.currency":
		ensureDraftVariantCostPrice(variant)
		variant.CostPrice.Currency = strings.TrimSpace(edit.StringValue)
	case strings.HasPrefix(subfield, "attributes."):
		key := strings.TrimSpace(strings.TrimPrefix(subfield, "attributes."))
		if key == "" {
			return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
		}
		if variant.Attributes == nil {
			variant.Attributes = map[string]string{}
		}
		variant.Attributes[key] = strings.TrimSpace(edit.StringValue)
	default:
		return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
	}
	return nil
}

func applyVariantCanonicalEdit(product *canonical.Product, index int, subfield string, edit DraftFieldEdit) error {
	for len(product.Variants) <= index {
		product.Variants = append(product.Variants, canonical.Variant{
			Attributes: map[string]canonical.Attribute{},
			Trace:      manualFieldTrace(),
		})
	}
	variant := &product.Variants[index]
	if variant.Attributes == nil {
		variant.Attributes = map[string]canonical.Attribute{}
	}
	variant.Trace = manualFieldTrace()
	switch {
	case subfield == "sku":
		variant.SKU = strings.TrimSpace(edit.StringValue)
	case subfield == "barcode":
		variant.Barcode = strings.TrimSpace(edit.StringValue)
	case subfield == "inventory":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].inventory requires number_value", index)
		}
		variant.Stock = int(*edit.NumberValue)
	case subfield == "is_default":
		value, err := parseBooleanEdit(edit, fmt.Sprintf("variants[%d].is_default", index))
		if err != nil {
			return err
		}
		variant.IsDefault = value
	case subfield == "main_image":
		url := strings.TrimSpace(edit.StringValue)
		if url == "" {
			variant.Images = nil
		} else if len(variant.Images) == 0 {
			variant.Images = []canonical.Image{{URL: url, Role: "variant", Trace: manualFieldTrace()}}
		} else {
			variant.Images[0] = canonical.Image{URL: url, Role: "variant", Trace: manualFieldTrace()}
		}
	case subfield == "price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].price.amount requires number_value", index)
		}
		ensureCanonicalVariantPrice(variant)
		variant.Price.Amount = *edit.NumberValue
	case subfield == "price.currency":
		ensureCanonicalVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case subfield == "cost_price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].cost_price.amount requires number_value", index)
		}
		ensureCanonicalVariantPrice(variant)
		variant.Price.CostPrice = *edit.NumberValue
	case subfield == "cost_price.currency":
		ensureCanonicalVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case strings.HasPrefix(subfield, "attributes."):
		key := strings.TrimSpace(strings.TrimPrefix(subfield, "attributes."))
		if key == "" {
			return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
		}
		variant.Attributes[key] = canonical.Attribute{
			Value: strings.TrimSpace(edit.StringValue),
			Trace: manualFieldTrace(),
		}
	default:
		return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
	}
	return nil
}

func ensureDraftVariant(draft *AmazonListingDraft, index int) {
	for len(draft.Variants) <= index {
		draft.Variants = append(draft.Variants, AmazonVariantDraft{})
	}
}

func ensureDraftVariantPrice(variant *AmazonVariantDraft) {
	if variant.Price == nil {
		variant.Price = &AmazonMoney{}
	}
}

func ensureDraftVariantCostPrice(variant *AmazonVariantDraft) {
	if variant.CostPrice == nil {
		variant.CostPrice = &AmazonMoney{}
	}
}

func ensureCanonicalVariantPrice(variant *canonical.Variant) {
	if variant.Price == nil {
		variant.Price = &canonical.PriceInfo{}
	}
}
