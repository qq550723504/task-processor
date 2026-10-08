package shein

import (
	"context"
	"encoding/json"
	"task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
)

func (c *OfficialClient) QueryProductSPU(ctx context.Context, credential storecenter.OfficialMerchantCredential, spu string) (model.ProductReadback, error) {
	if !boundedParameter(spu, 128) {
		return model.ProductReadback{}, ErrGoodsInvalid
	}
	payload, _ := json.Marshal(struct {
		SPU       string   `json:"spuName"`
		Languages []string `json:"languageList"`
	}{spu, []string{"en"}})
	raw, err := c.goodsPost(ctx, "/open-api/goods/spu-info", credential, payload)
	if err != nil {
		return model.ProductReadback{}, err
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			SPU           string `json:"spuName"`
			CategoryID    int64  `json:"categoryId"`
			ProductTypeID int64  `json:"productTypeId"`
			BrandCode     string `json:"brandCode"`
			SupplierCode  string `json:"supplierCode"`
			Names         []struct {
				Language string `json:"language"`
				Name     string `json:"productName"`
			} `json:"productMultiNameList"`
			SKCs []struct {
				Name         string `json:"skcName"`
				SupplierCode string `json:"supplierCode"`
				SKUs         []struct {
					Code        string `json:"skuCode"`
					SupplierSKU string `json:"supplierSku"`
				} `json:"skuInfoList"`
			} `json:"skcInfoList"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.SPU != spu || len(response.Info.SKCs) > 40 || len(response.Info.Names) > 5 {
		return model.ProductReadback{}, ErrGoodsUnavailable
	}
	v := response.Info
	out := model.ProductReadback{CategoryID: v.CategoryID, ProductTypeID: v.ProductTypeID, BrandCode: v.BrandCode, SupplierCode: v.SupplierCode, SKCSupplierCodes: map[string]string{}, Product: model.PublishResult{SPUName: v.SPU, ResponseHash: goodsHash(raw)}}
	for _, n := range v.Names {
		out.Names = append(out.Names, model.LanguageContent{Language: n.Language, Name: n.Name})
	}
	for _, s := range v.SKCs {
		if _, duplicate := out.SKCSupplierCodes[s.Name]; duplicate || len(s.SKUs) > 400 {
			return model.ProductReadback{}, ErrGoodsUnavailable
		}
		group := model.PublishedSKC{SKCName: s.Name}
		out.SKCSupplierCodes[s.Name] = s.SupplierCode
		for _, sku := range s.SKUs {
			group.SKUs = append(group.SKUs, model.PublishedSKU{SKUCode: sku.Code, SupplierSKU: sku.SupplierSKU})
		}
		out.Product.SKCs = append(out.Product.SKCs, group)
	}
	return out, nil
}
