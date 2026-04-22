package shein

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

type RevisionInput struct {
	SpuName                 *string                       `json:"spu_name,omitempty"`
	ProductNameEn           *string                       `json:"product_name_en,omitempty"`
	BrandName               *string                       `json:"brand_name,omitempty"`
	Description             *string                       `json:"description,omitempty"`
	SellingPoints           []string                      `json:"selling_points,omitempty"`
	CategoryName            *string                       `json:"category_name,omitempty"`
	CategoryPath            []string                      `json:"category_path,omitempty"`
	CategoryID              *int                          `json:"category_id,omitempty"`
	CategoryIDList          []int                         `json:"category_id_list,omitempty"`
	ProductTypeID           *int                          `json:"product_type_id,omitempty"`
	TopCategoryID           *int                          `json:"top_category_id,omitempty"`
	Images                  *common.ImageSet              `json:"images,omitempty"`
	ProductAttributes       []common.Attribute            `json:"product_attributes,omitempty"`
	ResolvedAttributes      []sheinpub.ResolvedAttribute  `json:"resolved_attributes,omitempty"`
	CategoryResolution      *CategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *AttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SKCRevisionPatch            `json:"skc_patches,omitempty"`
	RequestDraft            *sheinpub.RequestDraft        `json:"request_draft,omitempty"`
	ReviewNotes             []string                      `json:"review_notes,omitempty"`
}

type CategoryResolutionPatch struct {
	Status         *string  `json:"status,omitempty"`
	Source         *string  `json:"source,omitempty"`
	QueryText      *string  `json:"query_text,omitempty"`
	MatchedPath    []string `json:"matched_path,omitempty"`
	CategoryID     *int     `json:"category_id,omitempty"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	ProductTypeID  *int     `json:"product_type_id,omitempty"`
	TopCategoryID  *int     `json:"top_category_id,omitempty"`
	ReviewNotes    []string `json:"review_notes,omitempty"`
}

type AttributeResolutionPatch struct {
	Status             *string                      `json:"status,omitempty"`
	Source             *string                      `json:"source,omitempty"`
	CategoryID         *int                         `json:"category_id,omitempty"`
	TemplateCount      *int                         `json:"template_count,omitempty"`
	ResolvedCount      *int                         `json:"resolved_count,omitempty"`
	UnresolvedCount    *int                         `json:"unresolved_count,omitempty"`
	ResolvedAttributes []sheinpub.ResolvedAttribute `json:"resolved_attributes,omitempty"`
	ReviewNotes        []string                     `json:"review_notes,omitempty"`
}

type SaleAttributeResolutionPatch struct {
	Status                  *string                          `json:"status,omitempty"`
	Source                  *string                          `json:"source,omitempty"`
	RecommendCategoryReview *bool                            `json:"recommend_category_review,omitempty"`
	CategoryReviewReason    *string                          `json:"category_review_reason,omitempty"`
	PrimaryAttributeID      *int                             `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID    *int                             `json:"secondary_attribute_id,omitempty"`
	SKCAttributes           []sheinpub.ResolvedSaleAttribute `json:"skc_attributes,omitempty"`
	SKUAttributes           []sheinpub.ResolvedSaleAttribute `json:"sku_attributes,omitempty"`
	SelectionSummary        []string                         `json:"selection_summary,omitempty"`
	ReviewNotes             []string                         `json:"review_notes,omitempty"`
}

type SKCRevisionPatch struct {
	SupplierCode  string                          `json:"supplier_code"`
	SkcName       *string                         `json:"skc_name,omitempty"`
	SaleName      *string                         `json:"sale_name,omitempty"`
	MainImageURL  *string                         `json:"main_image_url,omitempty"`
	SaleAttribute *sheinpub.ResolvedSaleAttribute `json:"sale_attribute,omitempty"`
	SKUPatches    []SKURevisionPatch              `json:"sku_patches,omitempty"`
}

type SKURevisionPatch struct {
	SupplierSKU    string                           `json:"supplier_sku"`
	Attributes     map[string]string                `json:"attributes,omitempty"`
	BasePrice      *string                          `json:"base_price,omitempty"`
	CostPrice      *string                          `json:"cost_price,omitempty"`
	Currency       *string                          `json:"currency,omitempty"`
	StockCount     *int                             `json:"stock_count,omitempty"`
	MainImage      *string                          `json:"main_image,omitempty"`
	Barcode        *string                          `json:"barcode,omitempty"`
	SaleAttributes []sheinpub.ResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []sheinpub.SitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []sheinpub.StockInfo             `json:"stock_info_list,omitempty"`
}

type EditorRevisionSkeleton struct {
	Platform string         `json:"platform"`
	Actor    string         `json:"actor,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Shein    *RevisionInput `json:"shein,omitempty"`
}
