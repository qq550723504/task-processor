package sku

import (
	"fmt"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinimage "task-processor/internal/shein/api/image"
	productapi "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

type RuntimeInput struct {
	AmazonProduct      *model.Product
	Variants           []model.Product
	AttributeTemplates *sheinattribute.AttributeTemplateInfo
	AsinSkuMap         map[string]string
	StoreInfo          *listingruntime.StoreInfo
	ProfitRule         *listingruntime.ProfitRule
	SiteList           []productapi.SiteInfo
	Region             string
	ImageAPI           sheinimage.ImageAPI
}

func newRuntimeInput(ctx *sheinctx.TaskContext) *RuntimeInput {
	input := &RuntimeInput{
		AmazonProduct:      ctx.AmazonProduct,
		AttributeTemplates: ctx.AttributeTemplates,
		StoreInfo:          ctx.StoreInfo,
		ProfitRule:         ctx.ProfitRule,
		Region:             "",
		ImageAPI:           ctx.ImageAPI,
	}
	if ctx.Variants != nil {
		input.Variants = append([]model.Product(nil), (*ctx.Variants)...)
	}
	if ctx.AsinSkuMap != nil {
		input.AsinSkuMap = make(map[string]string, len(ctx.AsinSkuMap))
		for k, v := range ctx.AsinSkuMap {
			input.AsinSkuMap[k] = v
		}
	}
	if len(ctx.SiteList) > 0 {
		input.SiteList = append([]productapi.SiteInfo(nil), ctx.SiteList...)
	}
	if ctx.Task != nil {
		input.Region = ctx.Task.Region
	}
	return input
}

func (in *RuntimeInput) Validate() error {
	if in == nil {
		return fmt.Errorf("SKU runtime input is not initialized")
	}
	if in.AmazonProduct == nil {
		return fmt.Errorf("amazon product is not initialized")
	}
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if in.StoreInfo == nil {
		return fmt.Errorf("store info is not initialized")
	}
	if in.ProfitRule == nil {
		return fmt.Errorf("profit rule is not initialized")
	}
	if len(in.SiteList) == 0 || len(in.SiteList[0].SubSiteList) == 0 {
		return fmt.Errorf("site list is not initialized")
	}
	if in.ImageAPI == nil {
		return fmt.Errorf("image api is not initialized")
	}
	return nil
}

func (in *RuntimeInput) FindProductInfoByASIN(asin string) *model.Product {
	for i := range in.Variants {
		if in.Variants[i].Asin == asin {
			return &in.Variants[i]
		}
	}
	return in.AmazonProduct
}

func (in *RuntimeInput) SupplierSKUForASIN(asin string) string {
	if in == nil || in.AsinSkuMap == nil {
		return ""
	}
	return in.AsinSkuMap[asin]
}

func (in *RuntimeInput) FixedStockCount() int {
	if in == nil || in.StoreInfo == nil || in.StoreInfo.FixedStockCount == nil {
		return 0
	}
	return *in.StoreInfo.FixedStockCount
}

func (in *RuntimeInput) DefaultSubSite() string {
	if in == nil || len(in.SiteList) == 0 || len(in.SiteList[0].SubSiteList) == 0 {
		return ""
	}
	return in.SiteList[0].SubSiteList[0]
}
