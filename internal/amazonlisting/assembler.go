package amazonlisting

import (
	"strings"
	"time"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type assembler struct{}

func NewAssembler() Assembler {
	return &assembler{}
}

func (a *assembler) Assemble(task *Task, product *productenrich.ProductJSON, image *productimage.ImageProcessResult) *AmazonListingDraft {
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
		draft.CategoryPath = append(draft.CategoryPath, product.Category...)
		draft.BulletPoints = append(draft.BulletPoints, product.SellingPoints...)
		draft.SearchTerms = append(draft.SearchTerms, product.SEOKeywords...)
		for k, v := range product.Attributes {
			draft.Attributes[k] = v
		}
		if draft.Brand == "" {
			draft.Brand = product.Attributes["brand"]
		}
		if draft.ProductType == "" && len(product.Category) > 0 {
			draft.ProductType = product.Category[len(product.Category)-1]
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
			variant := AmazonVariantDraft{
				SKU:        v.SKU,
				Attributes: v.Attributes,
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
				variant.MainImage = v.Images[0]
			}
			draft.Variants = append(draft.Variants, variant)
		}
		draft.Images = &AmazonImageBundle{RawInputImages: append([]string(nil), product.Images...)}
	}

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
	}

	if draft.Pricing == nil {
		draft.Pricing = &AmazonPricingDraft{Currency: currencyByCountry(task.Request.Country)}
	}
	if draft.Images == nil {
		draft.Images = &AmazonImageBundle{RawInputImages: append([]string(nil), task.Request.ImageURLs...)}
	}
	return draft
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
