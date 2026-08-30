package listingadmin

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewGormStoreAPISupportsSheinWorkerStoreOperations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&listingStore{}))

	autoLogin := true
	row := listingStore{
		TenantID:        246,
		StoreID:         "SHEIN-ORIGINAL",
		Name:            "SHEIN Store",
		Username:        "merchant",
		Password:        "secret",
		Platform:        "SHEIN",
		EnableAutoLogin: &autoLogin,
	}
	require.NoError(t, db.Create(&row).Error)

	storeAPI := NewGormStoreAPI(NewGormStoreRepository(db))
	_, err = storeAPI.GetStoreCookie(row.ID)
	require.Error(t, err)

	page, err := storeAPI.PageStores(&StorePageReqDTO{Platform: "SHEIN", PageNo: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	require.Equal(t, row.ID, page.List[0].ID)
	require.Equal(t, "SHEIN-ORIGINAL", page.List[0].StoreID)

	updated, err := storeAPI.UpdateStoreId(&StoreIdUpdateReqDTO{ID: row.ID, StoreID: "SHEIN-ACTUAL"})
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = storeAPI.UpdateStoreStatus(&StoreStatusUpdateReqDTO{ID: row.ID, Status: 1, Remark: "duplicate store"})
	require.NoError(t, err)
	require.True(t, updated)

	store, err := NewGormStoreRepository(db).FindStoreByID(t.Context(), row.ID)
	require.NoError(t, err)
	require.Equal(t, "SHEIN-ACTUAL", store.StoreID)
	require.Equal(t, int16(1), store.Status)
	require.Equal(t, "duplicate store", store.Remark)
}
