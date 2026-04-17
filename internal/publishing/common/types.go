package common

type Variant struct {
	SKU        string            `json:"sku,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Price      *Price            `json:"price,omitempty"`
	Stock      int               `json:"stock,omitempty"`
	Image      string            `json:"image,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	IsDefault  bool              `json:"is_default,omitempty"`
}

type Price struct {
	Currency  string  `json:"currency,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	CostPrice float64 `json:"cost_price,omitempty"`
}

type ImageSet struct {
	MainImage    string   `json:"main_image,omitempty"`
	WhiteBgImage string   `json:"white_bg_image,omitempty"`
	Gallery      []string `json:"gallery,omitempty"`
	SourceImages []string `json:"source_images,omitempty"`
}

type Attribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type Site struct {
	MainSite string   `json:"main_site,omitempty"`
	SubSites []string `json:"sub_sites,omitempty"`
}
