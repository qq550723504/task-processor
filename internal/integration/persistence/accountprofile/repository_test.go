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
	require.NoError(t, db.AutoMigrate(&profileRow{}, &auditRow{}))
	repo, err := New(db)
	require.NoError(t, err)

	ctx := context.Background()
	empty, err := repo.Read(ctx, "org-a", "user-a")
	require.NoError(t, err)
	require.Equal(t, "user-a", empty.UserID)
	require.Equal(t, []string{}, empty.Platforms)
	require.Equal(t, []string{}, empty.Sites)
	require.Equal(t, []string{}, empty.Services)
	require.Nil(t, empty.UpdatedAt)

	saved, err := repo.Save(ctx, BusinessProfile{OrganizationID: "org-a", UserID: "user-a", UserRole: "品牌方", ShopSituation: "已有店铺", FactorySituation: "无工厂", Platforms: []string{"1688", "Amazon"}, Sites: []string{"中国", "美国"}, ShopType: "品牌店", Services: []string{"选品", "图片"}})
	require.NoError(t, err)
	require.NotNil(t, saved.UpdatedAt)

	read, err := repo.Read(ctx, "org-a", "user-a")
	require.NoError(t, err)
	require.Equal(t, saved, read)

	updated, err := repo.Save(ctx, BusinessProfile{OrganizationID: "org-a", UserID: "user-a", UserRole: "供应链", Platforms: []string{"SHEIN"}})
	require.NoError(t, err)
	require.Equal(t, "供应链", updated.UserRole)
	require.Empty(t, updated.ShopSituation)
	require.Equal(t, []string{"SHEIN"}, updated.Platforms)
}

func TestRepositoryPersistsProfileAuditWithTheProfileMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&profileRow{}, &auditRow{}))
	repo, err := New(db)
	require.NoError(t, err)

	saved, err := repo.SaveWithAudit(context.Background(), BusinessProfile{OrganizationID: "org-a", UserID: "user-a", UserRole: "品牌方"}, AuditContext{OrganizationID: "org-a", ActorID: "actor-a"})
	require.NoError(t, err)
	require.NotNil(t, saved.UpdatedAt)

	events, next, err := repo.ListRecentAudit(context.Background(), "org-a", 20, "actor-a", "update", nil)
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, events, 1)
	require.Equal(t, int64(1), events[0].ID)
	require.Equal(t, AuditEvent{ID: events[0].ID, OrganizationID: "org-a", ActorID: "actor-a", UserID: "user-a", Operation: "update", Version: 1, CreatedAt: events[0].CreatedAt}, events[0])
	_, err = repo.SaveWithAudit(context.Background(), BusinessProfile{OrganizationID: "org-a", UserID: "user-a", UserRole: "供应链"}, AuditContext{OrganizationID: "org-a", ActorID: "actor-a"})
	require.NoError(t, err)
	events, next, err = repo.ListRecentAudit(context.Background(), "org-a", 20, "actor-a", "update", nil)
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, events, 2)
	require.Equal(t, int64(2), events[0].Version)
}

func TestRepositoryKeepsProfileStateAndAuditScopedByOrganization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&profileRow{}, &auditRow{}))
	repo, err := New(db)
	require.NoError(t, err)

	_, err = repo.SaveWithAudit(context.Background(), BusinessProfile{OrganizationID: "org-a", UserID: "user-a", UserRole: "品牌方"}, AuditContext{OrganizationID: "org-a", ActorID: "actor-a"})
	require.NoError(t, err)
	_, err = repo.SaveWithAudit(context.Background(), BusinessProfile{OrganizationID: "org-b", UserID: "user-a", UserRole: "供应链"}, AuditContext{OrganizationID: "org-b", ActorID: "actor-b"})
	require.NoError(t, err)

	profileA, err := repo.Read(context.Background(), "org-a", "user-a")
	require.NoError(t, err)
	profileB, err := repo.Read(context.Background(), "org-b", "user-a")
	require.NoError(t, err)
	require.Equal(t, "品牌方", profileA.UserRole)
	require.Equal(t, "供应链", profileB.UserRole)

	auditA, _, err := repo.ListRecentAudit(context.Background(), "org-a", 20, "", "update", nil)
	require.NoError(t, err)
	auditB, _, err := repo.ListRecentAudit(context.Background(), "org-b", 20, "", "update", nil)
	require.NoError(t, err)
	require.Len(t, auditA, 1)
	require.Len(t, auditB, 1)
	require.Equal(t, "org-a", auditA[0].OrganizationID)
	require.Equal(t, "org-b", auditB[0].OrganizationID)
}
