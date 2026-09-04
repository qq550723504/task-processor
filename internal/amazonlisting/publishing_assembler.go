package amazonlisting

import (
	"strings"
	"time"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type assembler struct{}

func NewAssembler() Assembler {
	return &assembler{}
}

func (a *assembler) Build(input DraftInput) (*AmazonListingDraft, error) {
	request := input.Request
	if request == nil {
		request = &GenerateRequest{}
	}

	images := &AmazonImageBundle{}
	approvedAssetIDs := make([]string, 0, len(input.ApprovedAssets.Assets))
	for _, approved := range input.ApprovedAssets.Assets {
		url := strings.TrimSpace(approved.URL)
		if url == "" {
			continue
		}
		switch approved.Role {
		case productasset.RoleMain:
			if images.MainImage == "" {
				images.MainImage = url
				approvedAssetIDs = append(approvedAssetIDs, approved.ID)
			}
		case productasset.RoleWhiteBackground:
			if images.WhiteBgImage == "" {
				images.WhiteBgImage = url
				approvedAssetIDs = append(approvedAssetIDs, approved.ID)
			}
		case productasset.RoleGallery:
			images.GalleryImages = append(images.GalleryImages, url)
			approvedAssetIDs = append(approvedAssetIDs, approved.ID)
		}
	}
	if images.MainImage == "" {
		return nil, productasset.ErrApprovedAssetsNotReady
	}
	if images.WhiteBgImage == "" {
		// ImageAgent's main slot is rendered by the white-background capability.
		images.WhiteBgImage = images.MainImage
	}

	now := time.Now()
	draft := &AmazonListingDraft{
		TaskID:       input.TaskID,
		Status:       string(TaskStatusProcessing),
		Marketplace:  request.Marketplace,
		Country:      request.Country,
		Language:     request.Language,
		Title:        input.Snapshot.Title,
		Brand:        input.Snapshot.Brand,
		CategoryPath: append([]string(nil), input.Snapshot.CategoryPath...),
		Description:  input.Snapshot.Description,
		BulletPoints: append([]string(nil), input.Snapshot.SellingPoints...),
		SearchTerms:  append([]string(nil), input.Snapshot.SEOKeywords...),
		Attributes:   make(map[string]string, len(input.Snapshot.Attributes)+1),
		Images:       images,
		Pricing:      &AmazonPricingDraft{Currency: currencyByCountry(request.Country)},
		Source: AmazonSourceTrace{
			ProductKey:       request.ProductKey,
			SnapshotSources:  append([]catalog.SourceRecord(nil), input.Snapshot.Sources...),
			ApprovedAssetIDs: approvedAssetIDs,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, attribute := range input.Snapshot.Attributes {
		name := strings.TrimSpace(attribute.Name)
		if name != "" {
			draft.Attributes[name] = attribute.Value
		}
	}
	if hint := strings.TrimSpace(request.BrandHint); hint != "" {
		draft.Brand = hint
	}
	if draft.Brand != "" {
		draft.Attributes["brand"] = draft.Brand
	}
	if len(draft.CategoryPath) > 0 {
		draft.ProductType = draft.CategoryPath[len(draft.CategoryPath)-1]
	} else {
		draft.ProductType = draft.Title
	}
	applyTargetCategoryHint(draft, request)
	applySnapshotSpecifications(draft, input.Snapshot.Specifications)
	applySnapshotVariants(draft, input.Snapshot.Variants, request.Country)
	if input.Snapshot.Review != nil && input.Snapshot.Review.NeedsReview {
		draft.Review = &AmazonReviewReport{NeedsReview: true, Reasons: append([]string(nil), input.Snapshot.Review.Reasons...)}
	}
	return draft, nil
}

func applySnapshotSpecifications(draft *AmazonListingDraft, specifications *catalog.Specifications) {
	if draft == nil || specifications == nil {
		return
	}
	if specifications.Dimensions != nil {
		draft.Dimensions = &AmazonDimensions{
			Length: specifications.Dimensions.Length,
			Width:  specifications.Dimensions.Width,
			Height: specifications.Dimensions.Height,
			Unit:   specifications.Dimensions.Unit,
		}
	}
	if specifications.Weight != nil {
		draft.Weight = &AmazonWeight{Value: specifications.Weight.Value, Unit: specifications.Weight.Unit}
	}
	if specifications.Package != nil {
		draft.Package = &AmazonPackageInfo{Quantity: specifications.Package.Quantity}
		if specifications.Package.Dimensions != nil {
			draft.Package.Dimensions = &AmazonDimensions{
				Length: specifications.Package.Dimensions.Length,
				Width:  specifications.Package.Dimensions.Width,
				Height: specifications.Package.Dimensions.Height,
				Unit:   specifications.Package.Dimensions.Unit,
			}
		}
		if specifications.Package.Weight != nil {
			draft.Package.Weight = &AmazonWeight{Value: specifications.Package.Weight.Value, Unit: specifications.Package.Weight.Unit}
		}
	}
	for key, value := range specifications.Technical {
		draft.Attributes[key] = value
	}
}

func applySnapshotVariants(draft *AmazonListingDraft, variants []catalog.Variant, country string) {
	if draft == nil {
		return
	}
	for _, variant := range variants {
		converted := AmazonVariantDraft{
			SKU:        variant.SKU,
			Attributes: make(map[string]string, len(variant.Attributes)),
			Inventory:  variant.Stock,
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
		}
		for _, attribute := range variant.Attributes {
			name := strings.TrimSpace(attribute.Name)
			if name != "" {
				converted.Attributes[name] = attribute.Value
			}
		}
		if variant.Price != nil {
			currency := normalizeCurrency(variant.Price.Currency, country)
			converted.Price = &AmazonMoney{Currency: currency, Amount: variant.Price.Amount}
			if variant.Price.CostPrice > 0 {
				converted.CostPrice = &AmazonMoney{Currency: currency, Amount: variant.Price.CostPrice}
			}
		}
		draft.Variants = append(draft.Variants, converted)
	}
}

func applyTargetCategoryHint(draft *AmazonListingDraft, req *GenerateRequest) {
	if draft == nil || req == nil {
		return
	}
	path := parseCategoryHint(req.TargetCategoryHint)
	if len(path) == 0 {
		return
	}
	draft.CategoryPath = append([]string(nil), path...)
	draft.ProductType = path[len(path)-1]
}

func parseCategoryHint(hint string) []string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return nil
	}

	var parts []string
	switch {
	case strings.Contains(hint, ">"):
		parts = strings.Split(hint, ">")
	case strings.Contains(hint, "/"):
		parts = strings.Split(hint, "/")
	case strings.Contains(hint, "|"):
		parts = strings.Split(hint, "|")
	default:
		parts = []string{hint}
	}

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func currencyByCountry(string) string {
	return "USD"
}

func normalizeCurrency(currency, country string) string {
	if strings.TrimSpace(currency) == "" || strings.EqualFold(currency, "CNY") {
		return currencyByCountry(country)
	}
	return strings.ToUpper(currency)
}
