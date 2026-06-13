package amazonlisting

import (
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func ensureCanonicalProduct(task *Task) {
	if task == nil || task.Result == nil || task.Result.CanonicalProduct != nil {
		return
	}
	task.Result.CanonicalProduct = canonicalProductFromDraft(task.Result)
}

func applyCanonicalEdits(product *canonical.Product, edits []DraftFieldEdit) error {
	if product == nil {
		return nil
	}
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	if product.Attributes == nil {
		product.Attributes = map[string]canonical.Attribute{}
	}
	for _, edit := range edits {
		field := strings.TrimSpace(edit.Field)
		if index, subfield, ok := parseIndexedField(field, "variants"); ok {
			if err := applyVariantCanonicalEdit(product, index, subfield, edit); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(field, "attributes.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "attributes."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			product.Attributes[key] = canonical.Attribute{
				Value: strings.TrimSpace(edit.StringValue),
				Trace: manualFieldTrace(),
			}
			continue
		}
		if strings.HasPrefix(field, "specifications.technical.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "specifications.technical."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			ensureCanonicalSpecifications(product)
			if product.Specifications.Technical == nil {
				product.Specifications.Technical = map[string]string{}
			}
			product.Specifications.Technical[key] = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
			continue
		}
		switch field {
		case "title":
			product.Title = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["title"] = manualFieldTrace()
		case "brand":
			product.Brand = strings.TrimSpace(edit.StringValue)
			if product.Brand != "" {
				product.Attributes["brand"] = canonical.Attribute{
					Value: product.Brand,
					Trace: manualFieldTrace(),
				}
			}
			product.FieldTraces["brand"] = manualFieldTrace()
		case "description":
			product.Description = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["description"] = manualFieldTrace()
		case "category_path":
			product.CategoryPath = trimStringList(edit.StringList)
			product.FieldTraces["category_path"] = manualFieldTrace()
		case "bullet_points":
			product.SellingPoints = trimStringList(edit.StringList)
			product.FieldTraces["selling_points"] = manualFieldTrace()
		case "search_terms":
			product.SEOKeywords = trimStringList(edit.StringList)
			product.FieldTraces["seo_keywords"] = manualFieldTrace()
		case "dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.length requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Length = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.width requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Width = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.height requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Height = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("weight.value requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalWeight(product.Specifications)
			product.Specifications.Weight.Value = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "weight.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalWeight(product.Specifications)
			product.Specifications.Weight.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.quantity":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.quantity requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackage(product.Specifications)
			product.Specifications.Package.Quantity = int(*edit.NumberValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.length requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Length = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.width requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Width = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.height requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Height = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.weight.value requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageWeight(product.Specifications)
			product.Specifications.Package.Weight.Value = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.weight.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageWeight(product.Specifications)
			product.Specifications.Package.Weight.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		}
	}
	product.NeedsReview = canonicalProductNeedsReview(product)
	return nil
}
