package shein

import (
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

func NormalizeSubmitCollections(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.BrandSeriesList == nil {
		product.BrandSeriesList = []string{}
	}
	if product.MultiLanguageMakeupIngredientList == nil {
		product.MultiLanguageMakeupIngredientList = []any{}
	}
	if product.ProductVideoList == nil {
		product.ProductVideoList = []sheinproduct.ProductVideo{}
	}
	if product.PartInfoList == nil {
		product.PartInfoList = []any{}
	}
	if product.PLMPatternIDList == nil {
		product.PLMPatternIDList = []any{}
	}
	if product.SizeAttributeList == nil {
		product.SizeAttributeList = []sheinproduct.SizeAttribute{}
	}
	if product.BackSizeAttributeList == nil {
		product.BackSizeAttributeList = []any{}
	}
	if product.ProductCertificateList == nil {
		product.ProductCertificateList = []int{}
	}
	if product.CertificateList == nil {
		product.CertificateList = []int{}
	}
	if product.DelOtherCertificateSNList == nil {
		product.DelOtherCertificateSNList = []string{}
	}
	if product.CustomAttributeRelation == nil {
		product.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{}
	}
}

func NormalizeSubmitExtra(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	fromPageID := "product_publish"
	product.Extra.FromPageID = &fromPageID
	product.Extra.SwitchToSPUPic = false
	product.Extra.UseCVTransformImage = false
	product.Extra.TransformCVSizeImage = false
}

func FinalizeSubmitTransportFields(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.Extra.SPUTag == nil {
		product.Extra.SPUTag = []string{}
	}
	if product.Extra.ConfirmVolumeSKU == nil {
		product.Extra.ConfirmVolumeSKU = []string{}
	}
	if product.Extra.ConfirmWeightSKU == nil {
		product.Extra.ConfirmWeightSKU = []string{}
	}
	if product.Extra.ControlPriceData == nil {
		product.Extra.ControlPriceData = map[string]string{}
	}
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		if skc.SiteDetailImageInfoList == nil {
			skc.SiteDetailImageInfoList = []sheinproduct.SiteDetailImageInfo{}
		}
		if skc.SiteSpecImageInfoList == nil {
			skc.SiteSpecImageInfoList = []any{}
		}
		if skc.SKCScopeAttributeList == nil {
			skc.SKCScopeAttributeList = []any{}
		}
		if skc.ProofOfStockList == nil {
			skc.ProofOfStockList = []any{}
		}
		for skuIndex := range skc.SKUS {
			sku := &skc.SKUS[skuIndex]
			if sku.CompetingCostPriceImages == nil {
				sku.CompetingCostPriceImages = []any{}
			}
		}
	}
}
