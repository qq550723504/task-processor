package shein

import (
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

type AssemblerConfig struct {
	CategoryResolver      CategoryResolver
	AttributeResolver     AttributeResolver
	SaleAttributeResolver SaleAttributeResolver
	PricingPolicy         PricingPolicy
}

type Assembler interface {
	Build(req *BuildRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *Package
}

type assembler struct {
	categoryResolver      CategoryResolver
	attributeResolver     AttributeResolver
	saleAttributeResolver SaleAttributeResolver
	pricingPolicy         PricingPolicy
}

func NewAssembler(config AssemblerConfig) Assembler {
	return &assembler{
		categoryResolver:      config.CategoryResolver,
		attributeResolver:     config.AttributeResolver,
		saleAttributeResolver: config.SaleAttributeResolver,
		pricingPolicy:         config.PricingPolicy,
	}
}

func (a *assembler) Build(req *BuildRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *Package {
	if canonical == nil {
		return &Package{ReviewNotes: []string{"canonical product is empty"}}
	}
	log := sheinLogger("shein/assembler")

	images := common.BuildImages(canonical, image)
	spuName := common.WithBrandHint(canonical.Title, req.BrandHint)
	copy := buildSheinListingCopy(canonical, spuName)
	brand := common.ResolveBrand(req.BrandHint, canonical)
	variants := common.BuildVariants(canonical)
	productAttributes := common.BuildAttributes(canonical.Attributes)
	siteList := common.DefaultSites(req.Country)
	categoryName := common.LastCategory(canonical.CategoryPath)

	pkg := &Package{
		SpuName:           spuName,
		BrandName:         brand,
		ProductNameEn:     copy.Title,
		ProductNameMulti:  copy.Title,
		CategoryName:      categoryName,
		CategoryPath:      append([]string(nil), canonical.CategoryPath...),
		Description:       copy.Description,
		SellingPoints:     append([]string(nil), canonical.SellingPoints...),
		Attributes:        common.FlattenAttributes(canonical.Attributes),
		ProductAttributes: productAttributes,
		SiteList:          siteList,
		Images:            images,
		RequestDraft: &RequestDraft{
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
	if strings.TrimSpace(req.TargetCategoryHint) != "" {
		pkg.Metadata["target_category_hint"] = req.TargetCategoryHint
	}
	if a.categoryResolver != nil {
		log.WithFields(logrus.Fields{
			"title":         canonical.Title,
			"category_path": strings.Join(canonical.CategoryPath, " > "),
		}).Info("starting SHEIN category resolution")
		pkg.CategoryResolution = a.categoryResolver.Resolve(req, canonical, pkg)
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
		pkg.AttributeResolution = a.attributeResolver.Resolve(req, canonical, pkg)
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
		pkg.SaleAttributeResolution = a.saleAttributeResolver.Resolve(req, canonical, pkg)
		if pkg.SaleAttributeResolution != nil {
			log.WithFields(logrus.Fields{
				"status":                 pkg.SaleAttributeResolution.Status,
				"category_id":            pkg.SaleAttributeResolution.CategoryID,
				"primary_attribute_id":   pkg.SaleAttributeResolution.PrimaryAttributeID,
				"secondary_attribute_id": pkg.SaleAttributeResolution.SecondaryAttributeID,
			}).Info("completed SHEIN sale attribute resolution")
		}
	}
	if pkg.SaleAttributeResolution != nil && pkg.SaleAttributeResolution.RecommendCategoryReview && pkg.CategoryResolution != nil {
		if recommender, ok := a.categoryResolver.(categoryRecommender); ok {
			pkg.CategoryResolution.SuggestedCategory = recommender.SuggestAlternative(req, canonical, pkg)
			if suggested := pkg.CategoryResolution.SuggestedCategory; suggested != nil && suggested.CategoryID > 0 {
				pkg.ReviewNotes = append(pkg.ReviewNotes, "建议复核 SHEIN 类目，可尝试候选类目: "+strings.Join(suggested.MatchedPath, " > "))
			}
		}
	}
	NormalizeListingCopy(pkg, canonical, req.Language)
	groups := buildVariantGroups(variants, images, pkg.SaleAttributeResolution)
	pkg.SkcList = buildSKCs(groups)
	supplierCode := firstSupplierCode(pkg.SkcList)
	pkg.RequestDraft.SupplierCode = supplierCode
	pkg.RequestDraft.SKCList = buildRequestSKCs(groups, images, siteList, canonical, a.pricingPolicy)
	ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	pkg.PreviewProduct = BuildPreviewProduct(pkg)
	log.WithFields(logrus.Fields{
		"category_id": pkg.CategoryID,
		"skc_count":   len(pkg.SkcList),
		"sku_count":   countPackageSKUs(pkg.SkcList),
	}).Info("built SHEIN preview package")
	return pkg
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

func buildRequestSKCs(groups []variantGroup, images *common.ImageSet, siteList []common.Site, canonical *productenrich.CanonicalProduct, pricingPolicy PricingPolicy) []SKCRequestDraft {
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

func buildSKUDraft(variant common.Variant, canonical *productenrich.CanonicalProduct, mainImage string, siteList []common.Site, pricingPolicy PricingPolicy) SKUDraft {
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
	if canonical != nil && canonical.Specifications != nil {
		if canonical.Specifications.Weight != nil {
			draft.Weight = canonical.Specifications.Weight.Value
			draft.WeightUnit = canonical.Specifications.Weight.Unit
		}
		if canonical.Specifications.Dimensions != nil {
			draft.Length = common.FormatFloat(canonical.Specifications.Dimensions.Length)
			draft.Width = common.FormatFloat(canonical.Specifications.Dimensions.Width)
			draft.Height = common.FormatFloat(canonical.Specifications.Dimensions.Height)
			draft.LengthUnit = canonical.Specifications.Dimensions.Unit
		}
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
