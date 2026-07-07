package store

import "gorm.io/gorm"

func ApplySheinPODImageLookupStoreScopeForTest(db *gorm.DB, storeID int64) *gorm.DB {
	return applySheinPODImageLookupStoreScope(db, storeID)
}

func ApplySheinPODImageLookupQueryScopeForTest(db *gorm.DB, query string) *gorm.DB {
	return applySheinPODImageLookupQueryScope(db, query)
}
