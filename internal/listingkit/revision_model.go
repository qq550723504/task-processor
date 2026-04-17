package listingkit

import "time"

type ApplyRevisionRequest struct {
	Platform              string                `json:"platform"`
	Actor                 string                `json:"actor,omitempty"`
	Reason                string                `json:"reason,omitempty"`
	RestoreFromRevisionID string                `json:"restore_from_revision_id,omitempty"`
	Amazon                *AmazonRevisionInput  `json:"amazon,omitempty"`
	Shein                 *SheinRevisionInput   `json:"shein,omitempty"`
	Temu                  *TemuRevisionInput    `json:"temu,omitempty"`
	Walmart               *WalmartRevisionInput `json:"walmart,omitempty"`
}

type ListingKitRevisionSummary struct {
	UpdatedAt              time.Time                          `json:"updated_at"`
	UpdatedBy              string                             `json:"updated_by,omitempty"`
	Reason                 string                             `json:"reason,omitempty"`
	Platform               string                             `json:"platform,omitempty"`
	ActionType             string                             `json:"action_type,omitempty"`
	RestoredFromRevisionID string                             `json:"restored_from_revision_id,omitempty"`
	Timeline               *ListingKitRevisionTimelineSummary `json:"timeline,omitempty"`
}

type ListingKitRevisionRecord struct {
	RevisionID             string                             `json:"revision_id,omitempty"`
	UpdatedAt              time.Time                          `json:"updated_at"`
	UpdatedBy              string                             `json:"updated_by,omitempty"`
	Reason                 string                             `json:"reason,omitempty"`
	Platform               string                             `json:"platform,omitempty"`
	ActionType             string                             `json:"action_type,omitempty"`
	RestoredFromRevisionID string                             `json:"restored_from_revision_id,omitempty"`
	Timeline               *ListingKitRevisionTimelineSummary `json:"timeline,omitempty"`
	AppliedChanges         *RevisionDiffPreview               `json:"applied_changes,omitempty"`
	EditorContext          *SheinEditorContext                `json:"editor_context_snapshot,omitempty"`
}

type ListingKitRevisionTimelineSummary struct {
	Headline     string `json:"headline,omitempty"`
	Badge        string `json:"badge,omitempty"`
	RelationText string `json:"relation_text,omitempty"`
	ChangeCount  int    `json:"change_count,omitempty"`
}

type AmazonRevisionInput struct {
	Title        *string  `json:"title,omitempty"`
	Brand        *string  `json:"brand,omitempty"`
	BulletPoints []string `json:"bullet_points,omitempty"`
	Description  *string  `json:"description,omitempty"`
}

type SheinRevisionInput struct {
	SpuName                 *string                            `json:"spu_name,omitempty"`
	ProductNameEn           *string                            `json:"product_name_en,omitempty"`
	BrandName               *string                            `json:"brand_name,omitempty"`
	Description             *string                            `json:"description,omitempty"`
	SellingPoints           []string                           `json:"selling_points,omitempty"`
	CategoryName            *string                            `json:"category_name,omitempty"`
	CategoryPath            []string                           `json:"category_path,omitempty"`
	CategoryID              *int                               `json:"category_id,omitempty"`
	CategoryIDList          []int                              `json:"category_id_list,omitempty"`
	ProductTypeID           *int                               `json:"product_type_id,omitempty"`
	TopCategoryID           *int                               `json:"top_category_id,omitempty"`
	Images                  *PlatformImageSet                  `json:"images,omitempty"`
	ProductAttributes       []PlatformAttribute                `json:"product_attributes,omitempty"`
	ResolvedAttributes      []SheinResolvedAttribute           `json:"resolved_attributes,omitempty"`
	CategoryResolution      *SheinCategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *SheinAttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SheinSaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SheinSKCRevisionPatch            `json:"skc_patches,omitempty"`
	RequestDraft            *SheinRequestDraft                 `json:"request_draft,omitempty"`
	ReviewNotes             []string                           `json:"review_notes,omitempty"`
}

type SheinCategoryResolutionPatch struct {
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

type SheinAttributeResolutionPatch struct {
	Status             *string                  `json:"status,omitempty"`
	Source             *string                  `json:"source,omitempty"`
	CategoryID         *int                     `json:"category_id,omitempty"`
	TemplateCount      *int                     `json:"template_count,omitempty"`
	ResolvedCount      *int                     `json:"resolved_count,omitempty"`
	UnresolvedCount    *int                     `json:"unresolved_count,omitempty"`
	ResolvedAttributes []SheinResolvedAttribute `json:"resolved_attributes,omitempty"`
	ReviewNotes        []string                 `json:"review_notes,omitempty"`
}

type SheinSaleAttributeResolutionPatch struct {
	Status               *string                      `json:"status,omitempty"`
	Source               *string                      `json:"source,omitempty"`
	PrimaryAttributeID   *int                         `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID *int                         `json:"secondary_attribute_id,omitempty"`
	SKCAttributes        []SheinResolvedSaleAttribute `json:"skc_attributes,omitempty"`
	SKUAttributes        []SheinResolvedSaleAttribute `json:"sku_attributes,omitempty"`
	SelectionSummary     []string                     `json:"selection_summary,omitempty"`
	ReviewNotes          []string                     `json:"review_notes,omitempty"`
}

type SheinSKCRevisionPatch struct {
	SupplierCode  string                      `json:"supplier_code"`
	SkcName       *string                     `json:"skc_name,omitempty"`
	SaleName      *string                     `json:"sale_name,omitempty"`
	MainImageURL  *string                     `json:"main_image_url,omitempty"`
	SaleAttribute *SheinResolvedSaleAttribute `json:"sale_attribute,omitempty"`
	SKUPatches    []SheinSKURevisionPatch     `json:"sku_patches,omitempty"`
}

type SheinSKURevisionPatch struct {
	SupplierSKU    string                       `json:"supplier_sku"`
	Attributes     map[string]string            `json:"attributes,omitempty"`
	BasePrice      *string                      `json:"base_price,omitempty"`
	CostPrice      *string                      `json:"cost_price,omitempty"`
	Currency       *string                      `json:"currency,omitempty"`
	StockCount     *int                         `json:"stock_count,omitempty"`
	MainImage      *string                      `json:"main_image,omitempty"`
	Barcode        *string                      `json:"barcode,omitempty"`
	SaleAttributes []SheinResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []SheinSitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []SheinStockInfo             `json:"stock_info_list,omitempty"`
}

type TemuRevisionInput struct {
	GoodsName        *string           `json:"goods_name,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	BulletPoints     []string          `json:"bullet_points,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
}

type WalmartRevisionInput struct {
	ProductName      *string           `json:"product_name,omitempty"`
	Brand            *string           `json:"brand,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	LongDescription  *string           `json:"long_description,omitempty"`
	KeyFeatures      []string          `json:"key_features,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
}
