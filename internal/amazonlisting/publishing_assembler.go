package amazonlisting

import (
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
)

type assembler struct{}

func NewAssembler() Assembler {
	return &assembler{}
}

func (a *assembler) Assemble(task *Task, product *canonical.Product, image *productimage.ImageProcessResult) *AmazonListingDraft {
	now := time.Now()
	draft := &AmazonListingDraft{
		TaskID:       task.ID,
		Status:       string(TaskStatusProcessing),
		Marketplace:  task.Request.Marketplace,
		Country:      task.Request.Country,
		Language:     task.Request.Language,
		Attributes:   map[string]string{},
		CategoryPath: []string{},
		Source: AmazonSourceTrace{
			InputTextProvided: strings.TrimSpace(task.Request.Text) != "",
			InputImageCount:   len(task.Request.ImageURLs),
			ProductURL:        task.Request.ProductURL,
			UsedImageSources:  append([]string(nil), task.Request.ImageURLs...),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if hint := strings.TrimSpace(task.Request.BrandHint); hint != "" {
		draft.Brand = hint
		draft.Attributes["brand"] = hint
	}

	if product != nil {
		draft.Title = product.Title
		draft.Description = product.Description
		draft.CategoryPath = append(draft.CategoryPath, product.CategoryPath...)
		draft.BulletPoints = append(draft.BulletPoints, product.SellingPoints...)
		draft.SearchTerms = append(draft.SearchTerms, product.SEOKeywords...)
		for k, v := range product.Attributes {
			draft.Attributes[k] = v.Value
		}
		if draft.Brand == "" {
			draft.Brand = product.Brand
		}
		if draft.ProductType == "" && len(product.CategoryPath) > 0 {
			draft.ProductType = product.CategoryPath[len(product.CategoryPath)-1]
		}
		if draft.ProductType == "" {
			draft.ProductType = product.Title
		}
		if product.Specifications != nil {
			if product.Specifications.Dimensions != nil {
				draft.Dimensions = &AmazonDimensions{
					Length: product.Specifications.Dimensions.Length,
					Width:  product.Specifications.Dimensions.Width,
					Height: product.Specifications.Dimensions.Height,
					Unit:   product.Specifications.Dimensions.Unit,
				}
			}
			if product.Specifications.Weight != nil {
				draft.Weight = &AmazonWeight{
					Value: product.Specifications.Weight.Value,
					Unit:  product.Specifications.Weight.Unit,
				}
			}
			if product.Specifications.Package != nil {
				draft.Package = &AmazonPackageInfo{Quantity: product.Specifications.Package.Quantity}
				if product.Specifications.Package.Dimensions != nil {
					draft.Package.Dimensions = &AmazonDimensions{
						Length: product.Specifications.Package.Dimensions.Length,
						Width:  product.Specifications.Package.Dimensions.Width,
						Height: product.Specifications.Package.Dimensions.Height,
						Unit:   product.Specifications.Package.Dimensions.Unit,
					}
				}
				if product.Specifications.Package.Weight != nil {
					draft.Package.Weight = &AmazonWeight{
						Value: product.Specifications.Package.Weight.Value,
						Unit:  product.Specifications.Package.Weight.Unit,
					}
				}
			}
		}
		for _, v := range product.Variants {
			attributes := make(map[string]string, len(v.Attributes))
			for key, value := range v.Attributes {
				attributes[key] = value.Value
			}
			variant := AmazonVariantDraft{
				SKU:        v.SKU,
				Attributes: attributes,
				Inventory:  v.Stock,
				Barcode:    v.Barcode,
				IsDefault:  v.IsDefault,
			}
			if v.Price != nil {
				variant.Price = &AmazonMoney{Currency: normalizeCurrency(v.Price.Currency, task.Request.Country), Amount: v.Price.Amount}
				if v.Price.CostPrice > 0 {
					variant.CostPrice = &AmazonMoney{Currency: normalizeCurrency(v.Price.Currency, task.Request.Country), Amount: v.Price.CostPrice}
				}
			}
			if len(v.Images) > 0 {
				variant.MainImage = v.Images[0].URL
			}
			draft.Variants = append(draft.Variants, variant)
		}
		rawImages := make([]string, 0, len(product.Images))
		for _, image := range product.Images {
			rawImages = append(rawImages, image.URL)
		}
		draft.Images = &AmazonImageBundle{RawInputImages: rawImages}
	}

	applyTargetCategoryHint(draft, task.Request)

	if image != nil {
		if draft.Images == nil {
			draft.Images = &AmazonImageBundle{RawInputImages: append([]string(nil), task.Request.ImageURLs...)}
		}
		if image.MainImage != nil {
			draft.Images.MainImage = image.MainImage.URL
		}
		if image.WhiteBgImage != nil {
			draft.Images.WhiteBgImage = image.WhiteBgImage.URL
		}
		for _, asset := range image.GalleryImages {
			draft.Images.GalleryImages = append(draft.Images.GalleryImages, asset.URL)
		}
		if image.Review != nil && image.Review.NeedsReview {
			draft.Review = &AmazonReviewReport{
				NeedsReview: true,
				Reasons:     append([]string(nil), image.Review.Reasons...),
			}
		}
		if image.IPRisk != nil {
			draft.ListingIPRisk = mergeListingIPRisk(draft.ListingIPRisk, &IPRiskReport{
				Level:   image.IPRisk.Level,
				Score:   image.IPRisk.Score,
				Reasons: append([]string(nil), image.IPRisk.Reasons...),
			})
		}
	}

	if draft.Pricing == nil {
		draft.Pricing = &AmazonPricingDraft{Currency: currencyByCountry(task.Request.Country)}
	}
	if draft.Images == nil {
		draft.Images = &AmazonImageBundle{RawInputImages: append([]string(nil), task.Request.ImageURLs...)}
	}
	return draft
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
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func currencyByCountry(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "US", "":
		return "USD"
	default:
		return "USD"
	}
}

func normalizeCurrency(currency, country string) string {
	if strings.TrimSpace(currency) == "" || strings.EqualFold(currency, "CNY") {
		return currencyByCountry(country)
	}
	return strings.ToUpper(currency)
}

func mergeListingIPRisk(base *IPRiskReport, incoming *IPRiskReport) *IPRiskReport {
	if base == nil && incoming == nil {
		return nil
	}
	if base == nil {
		return &IPRiskReport{
			Level:   incoming.Level,
			Score:   incoming.Score,
			Reasons: append([]string(nil), incoming.Reasons...),
		}
	}
	if incoming == nil {
		return base
	}
	base.Score += incoming.Score
	if base.Score > 1 {
		base.Score = 1
	}
	if ipRiskPriority(incoming.Level) > ipRiskPriority(base.Level) {
		base.Level = incoming.Level
	}
	base.Reasons = uniqueSorted(append(base.Reasons, incoming.Reasons...))
	return base
}

func ipRiskPriority(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
