package listingadmin

import "strings"

func (s listingStore) toStore() Store {
	return Store{
		ID:                      s.ID,
		TenantID:                s.TenantID,
		OwnerUserID:             strings.TrimSpace(firstNonEmptyStoreValue(s.OwnerUserID, s.CreatedBy, s.Creator)),
		CreatedBy:               strings.TrimSpace(firstNonEmptyStoreValue(s.CreatedBy, s.Creator)),
		UpdatedBy:               strings.TrimSpace(firstNonEmptyStoreValue(s.UpdatedBy, s.Updater)),
		StoreID:                 s.StoreID,
		Name:                    s.Name,
		Username:                s.Username,
		Password:                s.Password,
		LoginURL:                s.LoginURL,
		ShopType:                s.ShopType,
		Region:                  s.Region,
		Platform:                s.Platform,
		DailyLimit:              s.DailyLimit,
		DailyLimitType:          s.DailyLimitType,
		FixedStockCount:         s.FixedStockCount,
		SKUGenerateStrategy:     s.SKUGenerateStrategy,
		Prefix:                  s.Prefix,
		Suffix:                  s.Suffix,
		Proxy:                   s.Proxy,
		EnableAutoListing:       s.EnableAutoListing,
		EnableAutoLogin:         s.EnableAutoLogin,
		EnableDraft:             s.EnableDraft,
		EnableAutoPrice:         s.EnableAutoPrice,
		EnableRebargain:         s.EnableRebargain,
		TemuPriceRejectStrategy: s.TemuPriceRejectStrategy,
		PriceType:               s.PriceType,
		Remark:                  s.Remark,
		Status:                  s.Status,
		ValidFrom:               s.ValidFrom,
		ValidUntil:              s.ValidUntil,
		Expired:                 s.Expired,
		DedicatedQueueEnabled:   s.DedicatedQueueEnabled,
		CreateTime:              s.CreateTime,
		UpdateTime:              s.UpdateTime,
	}
}

func listingStoreFromStore(store *Store) listingStore {
	if store == nil {
		return listingStore{}
	}
	return listingStore{
		ID:                      store.ID,
		TenantID:                store.TenantID,
		OwnerUserID:             strings.TrimSpace(store.OwnerUserID),
		StoreID:                 strings.TrimSpace(store.StoreID),
		Name:                    strings.TrimSpace(store.Name),
		Username:                strings.TrimSpace(store.Username),
		Password:                store.Password,
		LoginURL:                strings.TrimSpace(store.LoginURL),
		ShopType:                strings.TrimSpace(store.ShopType),
		Region:                  strings.TrimSpace(store.Region),
		Platform:                strings.TrimSpace(store.Platform),
		DailyLimit:              store.DailyLimit,
		DailyLimitType:          strings.TrimSpace(store.DailyLimitType),
		FixedStockCount:         store.FixedStockCount,
		SKUGenerateStrategy:     strings.TrimSpace(store.SKUGenerateStrategy),
		Prefix:                  strings.TrimSpace(store.Prefix),
		Suffix:                  strings.TrimSpace(store.Suffix),
		Proxy:                   strings.TrimSpace(store.Proxy),
		EnableAutoListing:       store.EnableAutoListing,
		EnableAutoLogin:         store.EnableAutoLogin,
		EnableDraft:             store.EnableDraft,
		EnableAutoPrice:         store.EnableAutoPrice,
		EnableRebargain:         store.EnableRebargain,
		TemuPriceRejectStrategy: strings.TrimSpace(store.TemuPriceRejectStrategy),
		PriceType:               strings.TrimSpace(store.PriceType),
		Remark:                  strings.TrimSpace(store.Remark),
		Status:                  store.Status,
		ValidFrom:               store.ValidFrom,
		ValidUntil:              store.ValidUntil,
		Expired:                 store.Expired,
		DedicatedQueueEnabled:   store.DedicatedQueueEnabled,
		CreatedBy:               strings.TrimSpace(store.CreatedBy),
		UpdatedBy:               strings.TrimSpace(store.UpdatedBy),
	}
}

func applyStoreCreateDefaults(row *listingStore) {
	if row == nil {
		return
	}
	if row.DailyLimitType == "" {
		row.DailyLimitType = "SPU"
	}
	if row.Region == "" {
		row.Region = "US"
	}
	if row.Creator == "" {
		row.Creator = row.CreatedBy
	}
	if row.Updater == "" {
		row.Updater = row.UpdatedBy
	}
}

func firstNonEmptyStoreValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
