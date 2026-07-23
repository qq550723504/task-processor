package listingkit

import "time"

// SheinPODImageLookupIndex stores the searchable projection for one listing-kit
// task. Lookup keys are fixed-length SHA-256 hashes of values normalized with
// sheinpodimage.NormalizeSheinPODImageLookupQueryToken.
type SheinPODImageLookupIndex struct {
	TaskID string `json:"task_id" gorm:"primaryKey;type:varchar(36)"`

	TenantID string `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID   string `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	StoreID  int64  `json:"store_id" gorm:"index:idx_shein_pod_image_lookup_task_id_key,priority:1;index:idx_shein_pod_image_lookup_product_name_key,priority:1;index:idx_shein_pod_image_lookup_supplier_code_key,priority:1;index:idx_shein_pod_image_lookup_seller_sku_key,priority:1;index:idx_shein_pod_image_lookup_shein_spu_name_key,priority:1;index:idx_shein_pod_image_lookup_shein_version_key,priority:1;index:idx_shein_pod_image_lookup_ai_original_image_url_key,priority:1;index:idx_shein_pod_image_lookup_ai_original_image_key_key,priority:1;index:idx_shein_pod_image_lookup_sds_main_image_url_key,priority:1"`

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

	// Normalized values are retained without indexes as a collision guard. The
	// fixed-length lookup keys below are the only searchable B-tree members.
	NormalizedTaskID             string `json:"-" gorm:"type:text"`
	NormalizedProductName        string `json:"-" gorm:"type:text"`
	NormalizedSupplierCode       string `json:"-" gorm:"type:text"`
	NormalizedSellerSKU          string `json:"-" gorm:"type:text"`
	NormalizedSheinSPUName       string `json:"-" gorm:"type:text"`
	NormalizedSheinVersion       string `json:"-" gorm:"type:text"`
	NormalizedAIOriginalImageURL string `json:"-" gorm:"type:text"`
	NormalizedAIOriginalImageKey string `json:"-" gorm:"type:text"`
	NormalizedSDSMainImageURL    string `json:"-" gorm:"type:text"`

	TaskIDLookupKey             string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_task_id_key,priority:2"`
	ProductNameLookupKey        string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_product_name_key,priority:2"`
	SupplierCodeLookupKey       string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_supplier_code_key,priority:2"`
	SellerSKULookupKey          string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_seller_sku_key,priority:2"`
	SheinSPUNameLookupKey       string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_shein_spu_name_key,priority:2"`
	SheinVersionLookupKey       string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_shein_version_key,priority:2"`
	AIOriginalImageURLLookupKey string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_ai_original_image_url_key,priority:2"`
	AIOriginalImageKeyLookupKey string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_ai_original_image_key_key,priority:2"`
	SDSMainImageURLLookupKey    string `json:"-" gorm:"type:char(64);index:idx_shein_pod_image_lookup_sds_main_image_url_key,priority:2"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SheinPODImageLookupIndex) TableName() string {
	return "listingkit_shein_pod_image_indexes"
}
