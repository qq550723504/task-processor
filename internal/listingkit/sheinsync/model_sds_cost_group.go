package sheinsync

import "time"

type SheinSDSCostGroupRecord struct {
	ID              int64     `json:"id" gorm:"primaryKey"`
	TenantID        int64     `json:"tenant_id" gorm:"index:idx_listingkit_shein_sds_cost_groups_scope,priority:1;uniqueIndex:uk_listingkit_shein_sds_cost_groups_scope_key,priority:1"`
	StoreID         int64     `json:"store_id" gorm:"index:idx_listingkit_shein_sds_cost_groups_scope,priority:2;uniqueIndex:uk_listingkit_shein_sds_cost_groups_scope_key,priority:2"`
	GroupKey        string    `json:"group_key" gorm:"type:varchar(128);index;uniqueIndex:uk_listingkit_shein_sds_cost_groups_scope_key,priority:3"`
	GroupLabel      string    `json:"group_label,omitempty" gorm:"type:varchar(128)"`
	ManualCostPrice *float64  `json:"manual_cost_price,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SheinSDSCostGroupRecord) TableName() string {
	return "listingkit_shein_sds_cost_groups"
}

type SheinSourceSDSCostGroupRecord struct {
	GroupKey        string                             `json:"group_key"`
	GroupLabel      string                             `json:"group_label"`
	SourceCode      string                             `json:"source_code"`
	SKUCodes        []string                           `json:"sku_codes,omitempty"`
	SKUGroups       []SheinSourceSDSSKUCostGroupRecord `json:"sku_groups,omitempty"`
	LegacyGroupKeys []string                           `json:"legacy_group_keys,omitempty"`
	ProductCount    int64                              `json:"product_count"`
	Products        []SheinSyncedProductRecord         `json:"products,omitempty"`
	ManualCostPrice *float64                           `json:"manual_cost_price,omitempty"`
}

type SheinSourceSDSSKUCostGroupRecord struct {
	GroupKey        string                     `json:"group_key"`
	GroupLabel      string                     `json:"group_label"`
	SourceCode      string                     `json:"source_code"`
	SKUCode         string                     `json:"sku_code"`
	VariantLabel    string                     `json:"variant_label,omitempty"`
	SKUCodes        []string                   `json:"sku_codes,omitempty"`
	ProductCount    int64                      `json:"product_count,omitempty"`
	Products        []SheinSyncedProductRecord `json:"products,omitempty"`
	LegacyGroupKeys []string                   `json:"legacy_group_keys,omitempty"`
	ManualCostPrice *float64                   `json:"manual_cost_price,omitempty"`
}
