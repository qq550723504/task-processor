package listingkit

type SDSSyncOptions struct {
	VariantID              int64                  `json:"variant_id,omitempty"`
	ParentProductID        int64                  `json:"parent_product_id,omitempty"`
	PrototypeGroupID       int64                  `json:"prototype_group_id,omitempty"`
	LayerID                string                 `json:"layer_id,omitempty"`
	DesignType             string                 `json:"design_type,omitempty"`
	FitLevel               float64                `json:"fit_level,omitempty"`
	ResizeMode             int                    `json:"resize_mode,omitempty"`
	ProductName            string                 `json:"product_name,omitempty"`
	ProductSKU             string                 `json:"product_sku,omitempty"`
	ProductEnglishName     string                 `json:"product_english_name,omitempty"`
	CategoryPath           []string               `json:"category_path,omitempty"`
	Material               string                 `json:"material,omitempty"`
	MaterialDescription    string                 `json:"material_description,omitempty"`
	ProductionProcess      string                 `json:"production_process,omitempty"`
	ProductPerformance     string                 `json:"product_performance,omitempty"`
	ApplicableScenarios    string                 `json:"applicable_scenarios,omitempty"`
	WashingInstructions    string                 `json:"washing_instructions,omitempty"`
	SpecialDescription     string                 `json:"special_description,omitempty"`
	ProductSize            string                 `json:"product_size,omitempty"`
	PackagingSpecification string                 `json:"packaging_specification,omitempty"`
	DesignArea             string                 `json:"design_area,omitempty"`
	PictureRequest         string                 `json:"picture_request,omitempty"`
	IsElectricity          *int                   `json:"is_electricity,omitempty"`
	VariantSKU             string                 `json:"variant_sku,omitempty"`
	VariantSize            string                 `json:"variant_size,omitempty"`
	VariantColor           string                 `json:"variant_color,omitempty"`
	VariantPrice           float64                `json:"variant_price,omitempty"`
	VariantWeight          float64                `json:"variant_weight,omitempty"`
	ProductionCycle        int                    `json:"production_cycle,omitempty"`
	BlankDesignURL         string                 `json:"blank_design_url,omitempty"`
	TemplateImageURL       string                 `json:"template_image_url,omitempty"`
	MaskImageURL           string                 `json:"mask_image_url,omitempty"`
	PrintableWidth         int                    `json:"printable_width,omitempty"`
	PrintableHeight        int                    `json:"printable_height,omitempty"`
	MockupImageURLs        []string               `json:"mockup_image_urls,omitempty"`
	StyleID                string                 `json:"style_id,omitempty"`
	StyleName              string                 `json:"style_name,omitempty"`
	Variants               []SDSSyncVariantOption `json:"variants,omitempty"`
}

type SDSSyncVariantOption struct {
	VariantID        int64    `json:"variant_id,omitempty"`
	VariantSKU       string   `json:"variant_sku,omitempty"`
	Size             string   `json:"size,omitempty"`
	Color            string   `json:"color,omitempty"`
	Price            float64  `json:"price,omitempty"`
	Weight           float64  `json:"weight,omitempty"`
	BoxLength        float64  `json:"box_length,omitempty"`
	BoxWidth         float64  `json:"box_width,omitempty"`
	BoxHeight        float64  `json:"box_height,omitempty"`
	ProductionCycle  int      `json:"production_cycle,omitempty"`
	PrototypeGroupID int64    `json:"prototype_group_id,omitempty"`
	LayerID          string   `json:"layer_id,omitempty"`
	TemplateImageURL string   `json:"template_image_url,omitempty"`
	MaskImageURL     string   `json:"mask_image_url,omitempty"`
	BlankDesignURL   string   `json:"blank_design_url,omitempty"`
	MockupImageURL   string   `json:"mockup_image_url,omitempty"`
	MockupImageURLs  []string `json:"mockup_image_urls,omitempty"`
}
