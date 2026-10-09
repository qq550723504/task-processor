package storecenter

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"time"
)

type productExecutionAccess struct {
	value ProductExecutionAuthorization
	err   error
}

func (a *productExecutionAccess) AuthorizeProductExecution(context.Context, ProductExecutionSubject) (ProductExecutionAuthorization, error) {
	return a.value, a.err
}
func TestProductExecutionRequiresCurrentOriginalMemberGrantServiceAndConnection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, db.AutoMigrate(&workbenchStoreRecord{}, &storeMemberGrantRow{}, &officialConnectionRow{}, &officialAttemptRow{}))
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	id, attemptID := uuid.NewString(), uuid.NewString()
	active := "active"
	require.NoError(t, db.Create(&workbenchStoreRecord{ID: id, OrganizationID: "org-a", Platform: "shein", RecordStatus: "active", ServiceStatus: &active, ServiceStartedAt: &start, ServiceExpiresAt: &end, ConnectionRef: attemptID, Version: 3}).Error)
	require.NoError(t, db.Create(&storeMemberGrantRow{OrganizationID: "org-a", StoreID: id, MemberID: "member-a", Active: true, Version: 1, UpdatedBy: "actor-a", UpdatedAt: now}).Error)
	require.NoError(t, db.Create(&officialConnectionRow{OrganizationID: "org-a", StoreID: id, AttemptID: attemptID, Version: 2, Status: "connected"}).Error)
	require.NoError(t, db.Create(&officialAttemptRow{OrganizationID: "org-a", StoreID: id, AttemptID: attemptID, ActorID: "actor-a", MemberID: "member-a", AppID: "app-a", AppVersion: "v1", ConnectionVersion: 2, State: "verified", KeyID: "key-a", Ciphertext: "sealed"}).Error)
	repo := &MemberScopedStoreRepository{db: db}
	authorization := &productExecutionAccess{value: ProductExecutionAuthorization{Access: StoreMemberAccess{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}, Allowed: true}}
	subject := ProductExecutionSubject{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a", Purpose: ProductPurposePublish}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	material, err := repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.NoError(t, err)
	require.EqualValues(t, 3, material.StoreVersion)
	require.Equal(t, "sealed", material.Attempt.Ciphertext)
	authorization.err = ErrDependencyUnavailable
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrDependencyUnavailable)
	authorization.err = nil
	authorization.value.Allowed = false
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrNotFound)
	authorization.value.Allowed = true
	authorization.value.Access.MemberID = "rejoined-member"
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrNotFound)
	authorization.value.Access.MemberID = "member-a"
	require.NoError(t, db.Model(&storeMemberGrantRow{}).Where("organization_id = ? AND store_id = ?", "org-a", id).Update("active", false).Error)
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrNotFound)
	authorization.value.Access.Administrator = true
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.NoError(t, err, "current administrator follows existing Store grant policy")
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, end)
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, db.Model(&officialConnectionRow{}).Where("organization_id = ?", "org-a").Update("status", "expired").Error)
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, db.Migrator().DropTable(&officialConnectionRow{}))
	_, err = repo.ReadProductExecution(ctx, subject, id, authorization, now)
	require.ErrorIs(t, err, ErrDependencyUnavailable, "database failure is not authoritative Store revocation")
}
