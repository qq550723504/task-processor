package template

import (
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
)

type TemplateQueryInput struct {
	ListingCommitID      string
	GoodsCommitID        string
	GoodsID              string
	CatID                int
	ListingCommitVersion string
}

func buildTemplateQueryInput(temuCtx *temucontext.TemuTaskContext) TemplateQueryInput {
	input := TemplateQueryInput{}
	if temuCtx.TemuProduct == nil {
		return input
	}

	goodsBasic := temuCtx.TemuProduct.GoodsBasic
	input.ListingCommitID = goodsBasic.ListingCommitID
	input.GoodsCommitID = goodsBasic.GoodsCommitID
	input.GoodsID = goodsBasic.GoodsID
	input.CatID = goodsBasic.CatID
	input.ListingCommitVersion = goodsBasic.ListingCommitVersion
	return input
}

func (i TemplateQueryInput) ToRequest() temutemplate.TemplateQueryRequest {
	request := temutemplate.TemplateQueryRequest{ClickType: "8"}
	if i.ListingCommitID != "" {
		request.ListingCommitID = i.ListingCommitID
	}
	if i.GoodsCommitID != "" {
		request.GoodsCommitID = i.GoodsCommitID
	}
	if i.GoodsID != "" {
		request.GoodsID = i.GoodsID
	}
	if i.CatID > 0 {
		request.CatID = i.CatID
	}
	if i.ListingCommitVersion != "" {
		request.ListingCommitVersion = i.ListingCommitVersion
	}
	return request
}
