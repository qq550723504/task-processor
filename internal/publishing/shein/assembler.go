package shein

import (
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

type AssemblerConfig struct {
	CategoryResolver      CategoryResolver
	AttributeResolver     AttributeResolver
	SaleAttributeResolver SaleAttributeResolver
	PricingPolicy         PricingPolicy
	TitleOptimizer        openaiclient.ChatCompleter
}

type Assembler interface {
	Build(req *BuildRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *Package
}

type assembler struct {
	categoryResolver      CategoryResolver
	attributeResolver     AttributeResolver
	saleAttributeResolver SaleAttributeResolver
	pricingPolicy         PricingPolicy
	titleOptimizer        openaiclient.ChatCompleter
}

func NewAssembler(config AssemblerConfig) Assembler {
	return &assembler{
		categoryResolver:      config.CategoryResolver,
		attributeResolver:     config.AttributeResolver,
		saleAttributeResolver: config.SaleAttributeResolver,
		pricingPolicy:         config.PricingPolicy,
		titleOptimizer:        config.TitleOptimizer,
	}
}

func (a *assembler) Build(req *BuildRequest, product *canonical.Product, image *productimage.ImageProcessResult) *Package {
	if product == nil {
		return &Package{ReviewNotes: []string{"canonical product is empty"}}
	}
	log := sheinLogger("shein/assembler")

	images := common.BuildImages(product, image)
	spuName := common.WithBrandHint(product.Title, req.BrandHint)
	copy := buildSheinListingCopy(product, spuName, a.titleOptimizer)
	brand := common.ResolveBrand(req.BrandHint, product)
	variants := common.BuildVariants(product)
	productAttributes := common.BuildAttributes(product.Attributes)
	siteList := common.DefaultSites(req.Country)
	categoryName := common.LastCategory(product.CategoryPath)

	pkg := &Package{
		SpuName:           spuName,
		BrandName:         brand,
		ProductNameEn:     copy.Title,
		ProductNameMulti:  copy.Title,
		TitleDiagnostics:  copy.TitleDiagnostics,
		CategoryName:      categoryName,
		CategoryPath:      append([]string(nil), product.CategoryPath...),
		Description:       copy.Description,
		SellingPoints:     append([]string(nil), product.SellingPoints...),
		Attributes:        common.FlattenAttributes(product.Attributes),
		ProductAttributes: productAttributes,
		SiteList:          siteList,
		Images:            images,
		DraftPayload: &RequestDraft{
			SpuName:               spuName,
			MultiLanguageNameList: localizedEnglishText(req.Language, copy.Title),
			MultiLanguageDescList: localizedEnglishText(req.Language, copy.Description),
			ProductAttributeList:  productAttributes,
			ImageInfo:             BuildImageDraft(images),
			SiteList:              siteList,
		},
		Metadata: map[string]string{
			"target_platform": "shein",
			"country":         req.Country,
			"language":        req.Language,
			"category_name":   categoryName,
		},
	}
	NormalizePackageSemanticFields(pkg)
	if copy.TitleDiagnostics != nil {
		pkg.Metadata["title_source"] = copy.TitleDiagnostics.Source
		if copy.TitleDiagnostics.PromptContaminated {
			pkg.Metadata["title_prompt_contaminated"] = "true"
		}
		if note := strings.TrimSpace(copy.TitleDiagnostics.ResolutionNote); note != "" {
			pkg.Metadata["title_resolution_note"] = note
		}
		if base := strings.TrimSpace(copy.TitleDiagnostics.SKCBaseTitle); base != "" {
			pkg.Metadata["title_skc_base"] = base
		}
	}
	if strings.TrimSpace(req.TargetCategoryHint) != "" {
		pkg.Metadata["target_category_hint"] = req.TargetCategoryHint
	}
	attachSourceFactReviewMetadata(pkg, product)
	if a.categoryResolver != nil {
		log.WithFields(logrus.Fields{
			"title":         product.Title,
			"category_path": strings.Join(product.CategoryPath, " > "),
		}).Info("starting SHEIN category resolution")
		pkg.CategoryResolution = a.categoryResolver.Resolve(req, product, pkg)
		ApplyCategoryResolution(pkg, pkg.CategoryResolution)
		if pkg.CategoryResolution != nil {
			log.WithFields(logrus.Fields{
				"status":       pkg.CategoryResolution.Status,
				"source":       pkg.CategoryResolution.Source,
				"category_id":  pkg.CategoryResolution.CategoryID,
				"matched_path": strings.Join(pkg.CategoryResolution.MatchedPath, " > "),
			}).Info("completed SHEIN category resolution")
		}
	}
	if a.attributeResolver != nil {
		log.WithField("category_id", pkg.CategoryID).Info("starting SHEIN display attribute resolution")
		pkg.AttributeResolution = a.attributeResolver.Resolve(req, product, pkg)
		ApplyAttributeResolution(pkg, pkg.AttributeResolution)
		if pkg.AttributeResolution != nil {
			log.WithFields(logrus.Fields{
				"status":           pkg.AttributeResolution.Status,
				"category_id":      pkg.AttributeResolution.CategoryID,
				"resolved_count":   pkg.AttributeResolution.ResolvedCount,
				"unresolved_count": pkg.AttributeResolution.UnresolvedCount,
				"template_count":   pkg.AttributeResolution.TemplateCount,
			}).Info("completed SHEIN display attribute resolution")
		}
	}
	if a.saleAttributeResolver != nil {
		log.WithField("category_id", pkg.CategoryID).Info("starting SHEIN sale attribute resolution")
		pkg.SaleAttributeResolution = a.saleAttributeResolver.Resolve(req, product, pkg)
		if pkg.SaleAttributeResolution != nil {
			log.WithFields(logrus.Fields{
				"status":                 pkg.SaleAttributeResolution.Status,
				"category_id":            pkg.SaleAttributeResolution.CategoryID,
				"primary_attribute_id":   pkg.SaleAttributeResolution.PrimaryAttributeID,
				"secondary_attribute_id": pkg.SaleAttributeResolution.SecondaryAttributeID,
			}).Info("completed SHEIN sale attribute resolution")
		}
	}
	NormalizeListingCopy(pkg, product, req.Language)
	groups := buildVariantGroups(copy.SKCTitleBase, variants, images, pkg.SaleAttributeResolution)
	pkg.SkcList = buildSKCs(groups)
	supplierCode := firstSupplierCode(pkg.SkcList)
	pkg.DraftPayload.SupplierCode = supplierCode
	pkg.DraftPayload.SKCList = buildRequestSKCs(groups, images, siteList, product, a.pricingPolicy)
	ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	SetPreviewPayload(pkg, BuildPreviewProduct(pkg))
	NormalizePackageSemanticFields(pkg)
	log.WithFields(logrus.Fields{
		"category_id": pkg.CategoryID,
		"skc_count":   len(pkg.SkcList),
		"sku_count":   countPackageSKUs(pkg.SkcList),
	}).Info("built SHEIN preview package")
	return pkg
}

func attachSourceFactReviewMetadata(pkg *Package, canonical *canonical.Product) {
	if pkg == nil || canonical == nil || !canonicalHas1688Source(canonical) {
		return
	}
	fields := sourceFactReviewFields(canonical)
	if len(fields) == 0 {
		return
	}
	if pkg.Metadata == nil {
		pkg.Metadata = map[string]string{}
	}
	pkg.Metadata["source_platform"] = "1688"
	pkg.Metadata["source_fact_review_required"] = "true"
	pkg.Metadata["source_fact_review_fields"] = strings.Join(fields, ",")
}

func canonicalHas1688Source(product *canonical.Product) bool {
	if product == nil {
		return false
	}
	for _, trace := range product.FieldTraces {
		for _, source := range trace.Sources {
			if source.Type == canonical.SourceProductURL && strings.Contains(strings.ToLower(source.Detail), "detail.1688.com") {
				return true
			}
		}
	}
	return false
}

func sourceFactReviewFields(product *canonical.Product) []string {
	if product == nil {
		return nil
	}
	criticalFields := []string{
		"title",
		"brand",
		"category_path",
		"description",
		"selling_points",
		"seo_keywords",
		"specifications",
	}
	fields := make([]string, 0, len(criticalFields))
	for _, field := range criticalFields {
		trace, ok := product.FieldTraces[field]
		if !ok || !trace.NeedsReview || !hasCanonicalSourceType(trace.Sources, canonical.SourceLLM) {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func hasCanonicalSourceType(sources []canonical.Source, want canonical.SourceType) bool {
	for _, source := range sources {
		if source.Type == want {
			return true
		}
	}
	return false
}

func countPackageSKUs(skcs []SKCPackage) int {
	total := 0
	for _, skc := range skcs {
		total += len(skc.SKUs)
	}
	return total
}

func buildSKCs(groups []variantGroup) []SKCPackage {
	if len(groups) == 0 {
		return nil
	}
	result := make([]SKCPackage, 0, len(groups))
	for _, group := range groups {
		result = append(result, SKCPackage{
			SkcName:      group.skcName,
			SaleName:     group.saleName,
			SupplierCode: group.supplierCode,
			MainImageURL: group.mainImageURL,
			Attributes:   common.CloneMap(group.attributes),
			SKUs:         append([]common.Variant(nil), group.skus...),
		})
	}
	return result
}

func buildRequestSKCs(groups []variantGroup, images *common.ImageSet, siteList []common.Site, canonical *canonical.Product, pricingPolicy PricingPolicy) []SKCRequestDraft {
	if len(groups) == 0 {
		return nil
	}
	result := make([]SKCRequestDraft, 0, len(groups))
	for idx, group := range groups {
		skus := make([]SKUDraft, 0, len(group.skus))
		for _, variant := range group.skus {
			skus = append(skus, buildSKUDraft(variant, canonical, common.FirstNonEmpty(variant.Image, group.mainImageURL, images.MainImage), siteList, pricingPolicy))
		}
		result = append(result, SKCRequestDraft{
			SkcName:      group.skcName,
			SaleName:     group.saleName,
			SupplierCode: group.supplierCode,
			Sort:         idx + 1,
			MultiLanguageNameList: []LocalizedText{
				{Language: "en", Name: group.skcName},
			},
			ImageInfo: BuildImageDraft(&common.ImageSet{
				MainImage:    group.mainImageURL,
				Gallery:      append([]string(nil), images.Gallery...),
				WhiteBgImage: images.WhiteBgImage,
			}),
			SKUList: skus,
		})
	}
	return result
}

func buildSKUDraft(variant common.Variant, canonical *canonical.Product, mainImage string, siteList []common.Site, pricingPolicy PricingPolicy) SKUDraft {
	draft := SKUDraft{
		SupplierSKU: variant.SKU,
		Attributes:  common.CloneMap(variant.Attributes),
		StockCount:  variant.Stock,
		MainImage:   mainImage,
		Barcode:     variant.Barcode,
		IsDefault:   variant.IsDefault,
	}
	if price := pricingPolicy.Apply(variant.Price); price != nil {
		draft.Currency = price.Currency
		draft.CostPrice = common.FormatFloat(price.CostPrice)
		draft.BasePrice = common.FormatFloat(price.Amount)
		draft.SitePriceList = buildSitePrices(price, siteList)
	}
	if variant.Weight != nil {
		draft.Weight = variant.Weight.Value
		draft.WeightUnit = variant.Weight.Unit
	} else if canonical != nil && canonical.Specifications != nil && canonical.Specifications.Weight != nil {
		draft.Weight = canonical.Specifications.Weight.Value
		draft.WeightUnit = canonical.Specifications.Weight.Unit
	}
	if variant.Dimensions != nil {
		draft.Length = common.FormatFloat(variant.Dimensions.Length)
		draft.Width = common.FormatFloat(variant.Dimensions.Width)
		draft.Height = common.FormatFloat(variant.Dimensions.Height)
		draft.LengthUnit = variant.Dimensions.Unit
	} else if canonical != nil && canonical.Specifications != nil && canonical.Specifications.Dimensions != nil {
		draft.Length = common.FormatFloat(canonical.Specifications.Dimensions.Length)
		draft.Width = common.FormatFloat(canonical.Specifications.Dimensions.Width)
		draft.Height = common.FormatFloat(canonical.Specifications.Dimensions.Height)
		draft.LengthUnit = canonical.Specifications.Dimensions.Unit
	}
	draft.StockInfoList = []StockInfo{{WarehouseCode: "DEFAULT", InventoryNum: variant.Stock}}
	return draft
}

func buildSitePrices(price *common.Price, siteList []common.Site) []SitePrice {
	if price == nil {
		return nil
	}
	subSite := "US"
	if len(siteList) > 0 && len(siteList[0].SubSites) > 0 {
		subSite = siteList[0].SubSites[0]
	}
	return []SitePrice{{SubSite: subSite, BasePrice: common.FormatFloat(price.Amount), Currency: price.Currency}}
}

func firstSupplierCode(skcs []SKCPackage) string {
	if len(skcs) == 0 {
		return ""
	}
	return skcs[0].SupplierCode
}
