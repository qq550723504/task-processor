package saleattribute

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	managementapi "task-processor/internal/infra/clients/management/api"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	sheinpub "task-processor/internal/publishing/shein"
	sheinctx "task-processor/internal/shein/context"
)

var errSaleAttributeRuntimeUnavailable = errors.New("sale attribute runtime context is unavailable")

type saleAttributeRuntimeResolver interface {
	Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution
}

type SaleAttributeResolutionHandler struct {
	resolver saleAttributeRuntimeResolver
}

func NewSaleAttributeResolutionHandler(resolver saleAttributeRuntimeResolver) *SaleAttributeResolutionHandler {
	return &SaleAttributeResolutionHandler{resolver: resolver}
}

func NewRuntimeSaleAttributeHandler(factory sheinpub.RuntimeAPIClientFactory, llm openaiclient.ChatCompleter, stores ...sheinpub.ResolutionCacheStore) *SaleAttributeResolutionHandler {
	return NewSaleAttributeResolutionHandler(sheinpub.NewRuntimeSaleAttributeResolver(factory, llm, stores...))
}

func (h *SaleAttributeResolutionHandler) Name() string {
	return "sale_attribute_resolution"
}

func (h *SaleAttributeResolutionHandler) Handle(ctx *sheinctx.TaskContext) error {
	if h == nil || h.resolver == nil || ctx == nil {
		if ctx != nil {
			ctx.SetSaleAttributeSelection(nil)
		}
		return nil
	}

	canonicalProduct, req, pkg, err := buildSaleAttributeResolutionInput(ctx)
	if err != nil {
		if errors.Is(err, errSaleAttributeRuntimeUnavailable) {
			ctx.SetSaleAttributeSelection(nil)
			return nil
		}
		ctx.SetSaleAttributeSelection(nil)
		return err
	}

	resolution := h.resolver.Resolve(req, canonicalProduct, pkg)
	if resolution == nil || resolution.PrimaryAttributeID <= 0 {
		ctx.SetSaleAttributeSelection(nil)
		return nil
	}

	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                   strings.TrimSpace(resolution.Source),
		PrimaryAttributeID:       resolution.PrimaryAttributeID,
		SecondaryAttributeID:     resolution.SecondaryAttributeID,
		PrimarySourceDimension:   strings.TrimSpace(resolution.PrimarySourceDimension),
		SecondarySourceDimension: strings.TrimSpace(resolution.SecondarySourceDimension),
	})
	return nil
}

func buildSaleAttributeResolutionInput(ctx *sheinctx.TaskContext) (*canonical.Product, *sheinpub.BuildRequest, *sheinpub.Package, error) {
	if ctx == nil {
		return nil, nil, nil, fmt.Errorf("task context is nil")
	}
	if ctx.AmazonProduct == nil {
		return nil, nil, nil, fmt.Errorf("amazon product is not initialized")
	}
	if ctx.ProductData == nil {
		return nil, nil, nil, fmt.Errorf("product data is not initialized")
	}
	if ctx.ProductData.CategoryID == 0 {
		return nil, nil, nil, fmt.Errorf("category id is not initialized")
	}
	if saleAttributeStoreID(ctx.StoreInfo) <= 0 {
		return nil, nil, nil, errSaleAttributeRuntimeUnavailable
	}

	canonicalProduct := &canonical.Product{
		Title:             strings.TrimSpace(ctx.AmazonProduct.Title),
		Brand:             strings.TrimSpace(ctx.AmazonProduct.Brand),
		Description:       strings.TrimSpace(ctx.AmazonProduct.Description),
		CategoryPath:      append([]string(nil), ctx.AmazonProduct.Categories...),
		VariantDimensions: buildCanonicalVariantDimensions(ctx.AmazonProduct),
		Variants:          buildCanonicalVariants(ctx.FilteredVariants()),
		Images:            buildCanonicalImages(ctx.AmazonProduct.Images),
	}

	req := &sheinpub.BuildRequest{
		Country:            resolveSaleAttributeCountry(ctx),
		Language:           "en",
		Text:               strings.TrimSpace(strings.Join([]string{ctx.AmazonProduct.Title, ctx.AmazonProduct.Description}, " ")),
		BrandHint:          strings.TrimSpace(ctx.AmazonProduct.Brand),
		TargetCategoryHint: strconv.Itoa(ctx.ProductData.CategoryID),
		SheinStoreID:       saleAttributeStoreID(ctx.StoreInfo),
		Context:            ctx.GetContext(),
	}

	pkg := &sheinpub.Package{
		SpuName:       strings.TrimSpace(ctx.AmazonProduct.Title),
		ProductNameEn: strings.TrimSpace(ctx.AmazonProduct.Title),
		BrandName:     strings.TrimSpace(ctx.AmazonProduct.Brand),
		CategoryID:    ctx.ProductData.CategoryID,
		CategoryPath:  append([]string(nil), ctx.AmazonProduct.Categories...),
		Description:   strings.TrimSpace(ctx.AmazonProduct.Description),
	}

	return canonicalProduct, req, pkg, nil
}

func buildCanonicalVariantDimensions(product *model.Product) []canonical.ScrapedVariantDimension {
	if product == nil || len(product.VariationsValues) == 0 {
		return nil
	}

	out := make([]canonical.ScrapedVariantDimension, 0, len(product.VariationsValues))
	for _, item := range product.VariationsValues {
		name := strings.TrimSpace(item.VariantName)
		if name == "" {
			continue
		}
		values := make([]string, 0, len(item.Values))
		for _, value := range item.Values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			values = append(values, value)
		}
		out = append(out, canonical.ScrapedVariantDimension{Name: name, Values: values})
	}
	return out
}

func buildCanonicalVariants(variants []model.Product) []canonical.Variant {
	if len(variants) == 0 {
		return nil
	}

	out := make([]canonical.Variant, 0, len(variants))
	for idx, item := range variants {
		attributes := extractCanonicalVariantAttributes(item)

		var images []canonical.Image
		if imageURL := strings.TrimSpace(item.ImageURL); imageURL != "" {
			images = []canonical.Image{{URL: imageURL}}
		}

		sku := firstNonEmpty(item.Asin, item.Title)
		out = append(out, canonical.Variant{
			SKU:        sku,
			Attributes: attributes,
			Images:     images,
			IsDefault:  idx == 0,
		})
	}
	return out
}

func extractCanonicalVariantAttributes(product model.Product) map[string]canonical.Attribute {
	attributes := map[string]canonical.Attribute{}

	for _, variation := range product.Variations {
		if strings.TrimSpace(variation.Asin) != "" && variation.Asin != product.Asin {
			continue
		}
		for key, value := range variation.Attributes {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			text := strings.TrimSpace(fmt.Sprint(value))
			if text == "" {
				continue
			}
			attributes[name] = canonical.Attribute{Value: text}
		}
	}

	return attributes
}

func buildCanonicalImages(urls []string) []canonical.Image {
	if len(urls) == 0 {
		return nil
	}

	out := make([]canonical.Image, 0, len(urls))
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		out = append(out, canonical.Image{URL: url})
	}
	return out
}

func resolveSaleAttributeCountry(ctx *sheinctx.TaskContext) string {
	if ctx == nil {
		return "US"
	}
	if ctx.Task != nil {
		if country := strings.ToUpper(strings.TrimSpace(ctx.Task.Region)); country != "" {
			return country
		}
	}
	if ctx.StoreInfo != nil {
		if country := strings.ToUpper(strings.TrimSpace(ctx.StoreInfo.Region)); country != "" {
			return country
		}
	}
	return "US"
}

func saleAttributeStoreID(storeInfo *managementapi.StoreRespDTO) int64 {
	if storeInfo == nil {
		return 0
	}
	return storeInfo.ID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
