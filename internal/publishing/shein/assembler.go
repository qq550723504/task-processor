package shein

import (
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

type AssemblerConfig struct {
	CategoryResolver      CategoryResolver
	AttributeResolver     AttributeResolver
	SaleAttributeResolver SaleAttributeResolver
}

type Assembler interface {
	Build(req *BuildRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *Package
}

type assembler struct {
	categoryResolver      CategoryResolver
	attributeResolver     AttributeResolver
	saleAttributeResolver SaleAttributeResolver
}

func NewAssembler(config AssemblerConfig) Assembler {
	return &assembler{
		categoryResolver:      config.CategoryResolver,
		attributeResolver:     config.AttributeResolver,
		saleAttributeResolver: config.SaleAttributeResolver,
	}
}

func (a *assembler) Build(req *BuildRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *Package {
	if canonical == nil {
		return &Package{ReviewNotes: []string{"canonical product is empty"}}
	}

	images := common.BuildImages(canonical, image)
	spuName := common.WithBrandHint(canonical.Title, req.BrandHint)
	brand := common.ResolveBrand(req.BrandHint, canonical)
	skcList := buildSKCs(canonical, images)
	productAttributes := common.BuildAttributes(canonical.Attributes)
	siteList := common.DefaultSites(req.Country)
	categoryName := common.LastCategory(canonical.CategoryPath)
	supplierCode := firstSupplierCode(skcList)

	pkg := &Package{
		SpuName:           spuName,
		BrandName:         brand,
		ProductNameEn:     spuName,
		ProductNameMulti:  common.FirstNonEmpty(strings.TrimSpace(canonical.Description), spuName),
		CategoryName:      categoryName,
		CategoryPath:      append([]string(nil), canonical.CategoryPath...),
		Description:       canonical.Description,
		SellingPoints:     append([]string(nil), canonical.SellingPoints...),
		Attributes:        common.FlattenAttributes(canonical.Attributes),
		ProductAttributes: productAttributes,
		SiteList:          siteList,
		SkcList:           skcList,
		Images:            images,
		RequestDraft: &RequestDraft{
			SpuName:      spuName,
			SupplierCode: supplierCode,
			MultiLanguageNameList: []LocalizedText{
				{Language: req.Language, Name: spuName},
				{Language: "en", Name: spuName},
			},
			MultiLanguageDescList: []LocalizedText{
				{Language: req.Language, Name: canonical.Description},
				{Language: "en", Name: canonical.Description},
			},
			ProductAttributeList: productAttributes,
			ImageInfo:            BuildImageDraft(images),
			SiteList:             siteList,
			SKCList:              buildRequestSKCs(canonical, images, siteList),
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
		pkg.CategoryResolution = a.categoryResolver.Resolve(req, canonical, pkg)
		ApplyCategoryResolution(pkg, pkg.CategoryResolution)
	}
	if a.attributeResolver != nil {
		pkg.AttributeResolution = a.attributeResolver.Resolve(req, canonical, pkg)
		ApplyAttributeResolution(pkg, pkg.AttributeResolution)
	}
	if a.saleAttributeResolver != nil {
		pkg.SaleAttributeResolution = a.saleAttributeResolver.Resolve(req, canonical, pkg)
		ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	}
	pkg.PreviewProduct = BuildPreviewProduct(pkg)
	return pkg
}

func buildSKCs(canonical *productenrich.CanonicalProduct, images *common.ImageSet) []SKCPackage {
	variants := common.BuildVariants(canonical)
	if len(variants) == 0 {
		return nil
	}
	result := make([]SKCPackage, 0, len(variants))
	for _, variant := range variants {
		saleName := common.FirstNonEmpty(variant.Attributes["color"], variant.Attributes["style"], variant.Attributes["size"], variant.SKU)
		mainImage := common.FirstNonEmpty(variant.Image, images.MainImage)
		result = append(result, SKCPackage{
			SkcName:      saleName,
			SaleName:     saleName,
			SupplierCode: variant.SKU,
			MainImageURL: mainImage,
			Attributes:   variant.Attributes,
			SKUs:         []common.Variant{variant},
		})
	}
	return result
}

func buildRequestSKCs(canonical *productenrich.CanonicalProduct, images *common.ImageSet, siteList []common.Site) []SKCRequestDraft {
	variants := common.BuildVariants(canonical)
	if len(variants) == 0 {
		return nil
	}
	result := make([]SKCRequestDraft, 0, len(variants))
	for idx, variant := range variants {
		saleName := common.FirstNonEmpty(variant.Attributes["color"], variant.Attributes["style"], variant.Attributes["size"], variant.SKU)
		mainImage := common.FirstNonEmpty(variant.Image, images.MainImage)
		result = append(result, SKCRequestDraft{
			SkcName:      saleName,
			SaleName:     saleName,
			SupplierCode: variant.SKU,
			Sort:         idx + 1,
			MultiLanguageNameList: []LocalizedText{
				{Language: "en", Name: saleName},
			},
			ImageInfo: BuildImageDraft(&common.ImageSet{
				MainImage:    mainImage,
				Gallery:      append([]string(nil), images.Gallery...),
				WhiteBgImage: images.WhiteBgImage,
			}),
			SKUList: []SKUDraft{
				buildSKUDraft(variant, canonical, mainImage, siteList),
			},
		})
	}
	return result
}

func buildSKUDraft(variant common.Variant, canonical *productenrich.CanonicalProduct, mainImage string, siteList []common.Site) SKUDraft {
	draft := SKUDraft{
		SupplierSKU: variant.SKU,
		Attributes:  common.CloneMap(variant.Attributes),
		StockCount:  variant.Stock,
		MainImage:   mainImage,
		Barcode:     variant.Barcode,
		IsDefault:   variant.IsDefault,
	}
	if variant.Price != nil {
		draft.Currency = variant.Price.Currency
		draft.CostPrice = common.FormatFloat(variant.Price.CostPrice)
		draft.BasePrice = common.FormatFloat(variant.Price.Amount)
		draft.SitePriceList = buildSitePrices(variant.Price, siteList)
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
