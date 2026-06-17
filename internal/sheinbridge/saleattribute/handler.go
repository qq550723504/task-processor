package saleattribute

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/core/logger"
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

	secondaryAttributeID := resolution.SecondaryAttributeID
	secondarySourceDimension := strings.TrimSpace(resolution.SecondarySourceDimension)
	secondaryUsable, secondaryReason := evaluateSecondarySelection(resolution)
	if !secondaryUsable {
		logger.GetGlobalLogger("shein/product").Warnf(
			"sale attribute secondary selection dropped: secondaryAttrID=%d secondaryDimension=%q skuAssignments=%d skuAttributes=%d reason=%s",
			resolution.SecondaryAttributeID,
			secondarySourceDimension,
			len(resolution.SKUValueAssignments),
			len(resolution.SKUAttributes),
			secondaryReason,
		)
		secondaryAttributeID = 0
		secondarySourceDimension = ""
	} else if resolution.SecondaryAttributeID > 0 {
		logger.GetGlobalLogger("shein/product").Infof(
			"sale attribute secondary selection kept: secondaryAttrID=%d secondaryDimension=%q skuAssignments=%d skuAttributes=%d reason=%s",
			resolution.SecondaryAttributeID,
			secondarySourceDimension,
			len(resolution.SKUValueAssignments),
			len(resolution.SKUAttributes),
			secondaryReason,
		)
	}

	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                   strings.TrimSpace(resolution.Source),
		PrimaryAttributeID:       resolution.PrimaryAttributeID,
		SecondaryAttributeID:     secondaryAttributeID,
		PrimarySourceDimension:   strings.TrimSpace(resolution.PrimarySourceDimension),
		SecondarySourceDimension: secondarySourceDimension,
	})
	return nil
}

func hasUsableSecondarySelection(resolution *sheinpub.SaleAttributeResolution) bool {
	usable, _ := evaluateSecondarySelection(resolution)
	return usable
}

func evaluateSecondarySelection(resolution *sheinpub.SaleAttributeResolution) (bool, string) {
	if resolution == nil || resolution.SecondaryAttributeID <= 0 {
		return false, "secondary attribute id is missing"
	}
	if len(resolution.SKUValueAssignments) > 0 {
		return true, fmt.Sprintf("resolved sku value assignments=%d", len(resolution.SKUValueAssignments))
	}
	for idx, attr := range resolution.SKUAttributes {
		if attr.AttributeID > 0 && attr.AttributeValueID != nil && *attr.AttributeValueID > 0 {
			return true, fmt.Sprintf("resolved sku attribute[%d] valueID=%d", idx, *attr.AttributeValueID)
		}
	}
	if strings.TrimSpace(resolution.SecondarySourceDimension) != "" && len(resolution.SKUAttributes) > 0 {
		return true, "secondary source dimension and sku attributes are present; defer value id repair to downstream mapping"
	}
	if len(resolution.SKUAttributes) == 0 {
		return false, "sku attributes are empty and sku value assignments are empty"
	}

	var invalidReasons []string
	for idx, attr := range resolution.SKUAttributes {
		switch {
		case attr.AttributeID <= 0:
			invalidReasons = append(invalidReasons, fmt.Sprintf("sku attribute[%d] has invalid attribute id", idx))
		case attr.AttributeValueID == nil:
			invalidReasons = append(invalidReasons, fmt.Sprintf("sku attribute[%d] value id is nil", idx))
		case *attr.AttributeValueID <= 0:
			invalidReasons = append(invalidReasons, fmt.Sprintf("sku attribute[%d] value id=%d", idx, *attr.AttributeValueID))
		}
	}
	if len(invalidReasons) == 0 {
		return false, "sku attributes present but none has a usable value id"
	}
	return false, strings.Join(invalidReasons, "; ")
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
