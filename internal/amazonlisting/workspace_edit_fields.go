package amazonlisting

import (
	"fmt"
	"strings"
)

func applyDraftEdits(draft *AmazonListingDraft, edits []DraftFieldEdit) error {
	if draft == nil {
		return fmt.Errorf("draft is nil")
	}
	for _, edit := range edits {
		field := strings.TrimSpace(edit.Field)
		if index, subfield, ok := parseIndexedField(field, "variants"); ok {
			if err := applyVariantDraftEdit(draft, index, subfield, edit); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(field, "attributes.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "attributes."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			draft.Attributes[key] = strings.TrimSpace(edit.StringValue)
			continue
		}
		if strings.HasPrefix(field, "specifications.technical.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "specifications.technical."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			draft.Attributes[key] = strings.TrimSpace(edit.StringValue)
			continue
		}
		switch field {
		case "title":
			draft.Title = strings.TrimSpace(edit.StringValue)
		case "brand":
			draft.Brand = strings.TrimSpace(edit.StringValue)
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			if draft.Brand != "" {
				draft.Attributes["brand"] = draft.Brand
			}
		case "description":
			draft.Description = strings.TrimSpace(edit.StringValue)
		case "category_path":
			draft.CategoryPath = trimStringList(edit.StringList)
			if len(draft.CategoryPath) > 0 {
				draft.ProductType = draft.CategoryPath[len(draft.CategoryPath)-1]
			}
		case "bullet_points":
			draft.BulletPoints = trimStringList(edit.StringList)
		case "search_terms":
			draft.SearchTerms = trimStringList(edit.StringList)
		case "images.main_image":
			ensureDraftImages(draft)
			draft.Images.MainImage = strings.TrimSpace(edit.StringValue)
		case "images.white_bg_image":
			ensureDraftImages(draft)
			draft.Images.WhiteBgImage = strings.TrimSpace(edit.StringValue)
		case "images.gallery":
			ensureDraftImages(draft)
			draft.Images.GalleryImages = trimStringList(edit.StringList)
		case "pricing.currency":
			ensureDraftPricing(draft)
			draft.Pricing.Currency = strings.TrimSpace(edit.StringValue)
		case "pricing.suggested_price":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.suggested_price requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.SuggestedPrice = *edit.NumberValue
		case "pricing.min_price":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.min_price requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.MinPrice = *edit.NumberValue
		case "pricing.source_cost":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.source_cost requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.SourceCost = *edit.NumberValue
		case "dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.length requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Length = *edit.NumberValue
		case "dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.width requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Width = *edit.NumberValue
		case "dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.height requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Height = *edit.NumberValue
		case "dimensions.unit":
			ensureDraftDimensions(draft)
			draft.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
		case "weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("weight.value requires number_value")
			}
			ensureDraftWeight(draft)
			draft.Weight.Value = *edit.NumberValue
		case "weight.unit":
			ensureDraftWeight(draft)
			draft.Weight.Unit = strings.TrimSpace(edit.StringValue)
		case "package.quantity":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.quantity requires number_value")
			}
			ensureDraftPackage(draft)
			draft.Package.Quantity = int(*edit.NumberValue)
		case "package.dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.length requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Length = *edit.NumberValue
		case "package.dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.width requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Width = *edit.NumberValue
		case "package.dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.height requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Height = *edit.NumberValue
		case "package.dimensions.unit":
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
		case "package.weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.weight.value requires number_value")
			}
			ensureDraftPackageWeight(draft)
			draft.Package.Weight.Value = *edit.NumberValue
		case "package.weight.unit":
			ensureDraftPackageWeight(draft)
			draft.Package.Weight.Unit = strings.TrimSpace(edit.StringValue)
		default:
			return fmt.Errorf("unsupported edit field: %s", field)
		}
	}
	return nil
}
