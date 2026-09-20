package accountprofile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryRoundTripsUserScopedProfile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&profileRow{}))
	repo, err := New(db)
	require.NoError(t, err)

	ctx := context.Background()
	empty, err := repo.Read(ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, "user-a", empty.UserID)
	require.Empty(t, empty.Platforms)
	require.Nil(t, empty.UpdatedAt)

	saved, err := repo.Save(ctx, BusinessProfile{UserID: "user-a", UserRole: "品牌方", ShopSituation: "已有店铺", FactorySituation: "无工厂", Platforms: []string{"1688", "Amazon"}, Sites: []string{"中国", "美国"}, ShopType: "品牌店", Services: []string{"选品", "图片"}})
	require.NoError(t, err)
	require.NotNil(t, saved.UpdatedAt)

	read, err := repo.Read(ctx, "user-a")
	require.NoError(t, err)
	require.Equal(t, saved, read)

	updated, err := repo.Save(ctx, BusinessProfile{UserID: "user-a", UserRole: "供应链", Platforms: []string{"SHEIN"}})
	require.NoError(t, err)
	require.Equal(t, "供应链", updated.UserRole)
	require.Empty(t, updated.ShopSituation)
	require.Equal(t, []string{"SHEIN"}, updated.Platforms)
}
