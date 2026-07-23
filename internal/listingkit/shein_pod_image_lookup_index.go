package listingkit

import "time"

// SheinPODImageLookupIndex stores the searchable projection for one listing-kit
// task. Normalized fields use sheinpodimage.NormalizeSheinPODImageLookupQueryToken
// when the projection is built.
type SheinPODImageLookupIndex struct {
	TaskID string `json:"task_id" gorm:"primaryKey;type:varchar(36)"`

	TenantID string `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID   string `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	StoreID  int64  `json:"store_id" gorm:"index:idx_shein_pod_image_lookup_task_id,priority:1;index:idx_shein_pod_image_lookup_product_name,priority:1;index:idx_shein_pod_image_lookup_supplier_code,priority:1;index:idx_shein_pod_image_lookup_seller_sku,priority:1;index:idx_shein_pod_image_lookup_shein_spu_name,priority:1;index:idx_shein_pod_image_lookup_shein_version,priority:1;index:idx_shein_pod_image_lookup_ai_original_image_url,priority:1;index:idx_shein_pod_image_lookup_ai_original_image_key,priority:1;index:idx_shein_pod_image_lookup_sds_main_image_url,priority:1"`

	Status              string   `json:"status,omitempty" gorm:"type:varchar(20)"`
	Prompt              string   `json:"prompt,omitempty" gorm:"type:text"`
	ProductName         string   `json:"product_name,omitempty" gorm:"type:text"`
	SupplierCode        string   `json:"supplier_code,omitempty" gorm:"type:text"`
	SellerSKU           string   `json:"seller_sku,omitempty" gorm:"type:text"`
	SheinSPUName        string   `json:"shein_spu_name,omitempty" gorm:"type:text"`
	SheinVersion        string   `json:"shein_version,omitempty" gorm:"type:text"`
	AIOriginalImageURL  string   `json:"ai_original_image_url,omitempty" gorm:"type:text"`
	AIOriginalImageKey  string   `json:"ai_original_image_key,omitempty" gorm:"type:text"`
	SDSMainImageURL     string   `json:"sds_main_image_url,omitempty" gorm:"type:text"`
	SDSGalleryImageURLs []string `json:"sds_gallery_image_urls,omitempty" gorm:"type:text;serializer:json"`

	NormalizedTaskID             string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_task_id,priority:2"`
	NormalizedProductName        string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_product_name,priority:2"`
	NormalizedSupplierCode       string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_supplier_code,priority:2"`
	NormalizedSellerSKU          string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_seller_sku,priority:2"`
	NormalizedSheinSPUName       string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_shein_spu_name,priority:2"`
	NormalizedSheinVersion       string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_shein_version,priority:2"`
	NormalizedAIOriginalImageURL string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_ai_original_image_url,priority:2"`
	NormalizedAIOriginalImageKey string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_ai_original_image_key,priority:2"`
	NormalizedSDSMainImageURL    string `json:"-" gorm:"type:text;index:idx_shein_pod_image_lookup_sds_main_image_url,priority:2"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SheinPODImageLookupIndex) TableName() string {
	return "listingkit_shein_pod_image_indexes"
}
