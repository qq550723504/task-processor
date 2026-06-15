package sheinsync

import "time"

type SheinSyncedProductRecord struct {
	ID                 int64                `json:"id" gorm:"primaryKey"`
	TenantID           int64                `json:"tenant_id" gorm:"index:idx_listingkit_shein_synced_products_scope,priority:1;uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:1"`
	StoreID            int64                `json:"store_id" gorm:"index:idx_listingkit_shein_synced_products_scope,priority:2;uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:2"`
	SPUName            string               `json:"spu_name,omitempty" gorm:"type:varchar(255)"`
	SPUCode            string               `json:"spu_code,omitempty" gorm:"type:varchar(128);index"`
	SKCName            string               `json:"skc_name,omitempty" gorm:"type:varchar(128);uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:3"`
	SKCCode            string               `json:"skc_code,omitempty" gorm:"type:varchar(128);index"`
	SupplierCode       string               `json:"supplier_code,omitempty" gorm:"type:varchar(128);index"`
	CategoryID         int64                `json:"category_id,omitempty" gorm:"index"`
	BrandName          string               `json:"brand_name,omitempty" gorm:"type:varchar(255)"`
	ProductNameMulti   string               `json:"product_name_multi,omitempty" gorm:"type:text"`
	MainImageURL       string               `json:"main_image_url,omitempty" gorm:"type:text"`
	SaleName           string               `json:"sale_name,omitempty" gorm:"type:varchar(255)"`
	ShelfStatus        string               `json:"shelf_status,omitempty" gorm:"type:varchar(64);index"`
	PublishTime        *time.Time           `json:"publish_time,omitempty"`
	FirstShelfTime     *time.Time           `json:"first_shelf_time,omitempty"`
	Currency           string               `json:"currency,omitempty" gorm:"type:varchar(16)"`
	PriceSnapshot      string               `json:"price_snapshot,omitempty" gorm:"type:text"`
	InventorySnapshot  string               `json:"inventory_snapshot,omitempty" gorm:"type:text"`
	SiteSnapshot       string               `json:"site_snapshot,omitempty" gorm:"type:text"`
	AutoCostPrice      *float64             `json:"auto_cost_price,omitempty"`
	ManualCostPrice    *float64             `json:"manual_cost_price,omitempty"`
	EffectiveCostPrice *float64             `json:"effective_cost_price,omitempty"`
	CostPriceSource    SheinCostPriceSource `json:"cost_price_source" gorm:"type:varchar(32);not null;default:'none'"`
	SyncVersion        string               `json:"sync_version,omitempty" gorm:"type:varchar(64);index"`
	LastSyncAt         *time.Time           `json:"last_sync_at,omitempty"`
	IsActive           bool                 `json:"is_active" gorm:"index;not null;default:true"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

func (SheinSyncedProductRecord) TableName() string {
	return "listingkit_shein_synced_products"
}

func ApplyEffectiveCostPrice(record *SheinSyncedProductRecord) {
	if record == nil {
		return
	}

	switch {
	case record.ManualCostPrice != nil:
		record.EffectiveCostPrice = sheinFloat64Ptr(*record.ManualCostPrice)
		record.CostPriceSource = SheinCostPriceSourceManual
	case record.AutoCostPrice != nil:
		record.EffectiveCostPrice = sheinFloat64Ptr(*record.AutoCostPrice)
		record.CostPriceSource = SheinCostPriceSourceAuto
	default:
		record.EffectiveCostPrice = nil
		record.CostPriceSource = SheinCostPriceSourceNone
	}
}
