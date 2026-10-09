package dataservicepersistence

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/dataservice"
	"task-processor/internal/product/collection"
)

func TestPostgresCredentialReplayCASRevocationAndQuotaLimits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("data621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, InstallSchema(db))
	store, err := NewCredentialRepository(ctx, db)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "original-grant"}
	key := dataservice.Credential{ID: uuid.NewString(), Scope: scope, Input: dataservice.KeyInput{Name: "test", ExpiresAt: time.Now().Add(time.Hour), DailyRows: 10, MonthlyCostFen: 50, Permissions: []string{dataservice.PermissionAcquire}}, Digest: collection.Digest("not-a-plaintext-secret"), Suffix: "test", State: "ACTIVE", Revision: 1, CreatedAt: time.Now().UTC()}
	cmd := uuid.NewString()
	hash := collection.Digest(key.Input)
	first, replayed, err := store.Create(ctx, key, cmd, hash)
	require.NoError(t, err)
	require.False(t, replayed)
	key.ID = uuid.NewString()
	again, replayed, err := store.Create(ctx, key, cmd, hash)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, first.ID, again.ID)
	require.Equal(t, scope, again.Scope)
	creation, err := store.Creation(ctx, scope, cmd)
	require.NoError(t, err)
	require.Equal(t, first.ID, creation.ID)
	_, err = store.Creation(ctx, collection.Scope{OrganizationID: "foreign-org", ActorID: scope.ActorID, MemberID: scope.MemberID}, cmd)
	require.ErrorIs(t, err, dataservice.ErrNotFound)
	_, _, err = store.Create(ctx, key, cmd, collection.Digest("changed"))
	require.ErrorIs(t, err, dataservice.ErrConflict)
	foreign, err := store.List(ctx, collection.Scope{OrganizationID: "org", ActorID: "other", MemberID: "other-grant"})
	require.NoError(t, err)
	require.Empty(t, foreign)
	require.NoError(t, db.Exec("INSERT INTO data_service_quota(organization_id,actor_id,key_id,window_kind,window_start,consumed_rows,reserved_rows,consumed_fen,reserved_fen) VALUES(?,?,?,'day',?,3,2,15,10),(?,?,?,'month',?,3,2,15,10)", scope.OrganizationID, scope.ActorID, first.ID, time.Now().UTC().Format("2006-01-02"), scope.OrganizationID, scope.ActorID, first.ID, time.Now().UTC().Format("2006-01")+"-01").Error)
	for _, limits := range []dataservice.KeyInput{
		{Name: "low rows", ExpiresAt: first.Input.ExpiresAt, DailyRows: 4, MonthlyCostFen: 50, Permissions: first.Input.Permissions},
		{Name: "low cost", ExpiresAt: first.Input.ExpiresAt, DailyRows: 10, MonthlyCostFen: 24, Permissions: first.Input.Permissions},
	} {
		_, err = store.Change(ctx, scope, first.ID, uuid.NewString(), collection.Digest(limits), 1, dataservice.KeyPatch{State: "ACTIVE", Limits: &limits})
		require.ErrorIs(t, err, dataservice.ErrConflict, "pending reservations must remain counted")
	}
	_, err = store.Change(ctx, scope, first.ID, uuid.NewString(), collection.Digest("stale"), 2, dataservice.KeyPatch{State: "DISABLED"})
	require.ErrorIs(t, err, dataservice.ErrConflict)
	patch := dataservice.KeyPatch{State: "REVOKED"}
	changeCmd := uuid.NewString()
	changeHash := collection.Digest(patch)
	revoked, err := store.Change(ctx, scope, first.ID, changeCmd, changeHash, 1, patch)
	require.NoError(t, err)
	require.Equal(t, "REVOKED", revoked.State)
	replayChange, err := store.Change(ctx, scope, first.ID, changeCmd, changeHash, 1, patch)
	require.NoError(t, err)
	require.Equal(t, revoked.Revision, replayChange.Revision)
	_, err = store.Change(ctx, scope, first.ID, uuid.NewString(), collection.Digest("resurrect"), 2, dataservice.KeyPatch{State: "ACTIVE"})
	require.ErrorIs(t, err, dataservice.ErrConflict)
	// A recreated grant cannot reactivate/rebind an old credential, but the
	// same creator must still be able to see and revoke their old metadata.
	oldBinding := key
	oldBinding.ID = uuid.NewString()
	oldBinding.Scope = scope
	oldBinding.State = "ACTIVE"
	oldBinding.Revision = 1
	_, _, err = store.Create(ctx, oldBinding, uuid.NewString(), collection.Digest(oldBinding.Input))
	require.NoError(t, err)
	current := scope
	current.MemberID = "recreated-grant"
	visible, err := store.List(ctx, current)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	oldHistory, err := store.History(ctx, current, "", 100)
	require.NoError(t, err)
	require.Len(t, oldHistory.Items, 1)
	require.Equal(t, first.ID, oldHistory.Items[0].ID)
	_, err = store.Change(ctx, current, oldBinding.ID, uuid.NewString(), collection.Digest("rebind old"), 1, dataservice.KeyPatch{State: "ACTIVE"})
	require.ErrorIs(t, err, dataservice.ErrForbidden)
	retired, err := store.Change(ctx, current, oldBinding.ID, uuid.NewString(), collection.Digest("revoke old"), 1, dataservice.KeyPatch{State: "REVOKED"})
	require.NoError(t, err)
	require.Equal(t, scope, retired.Scope, "original binding remains immutable")
	t.Run("expiry extension and create share the actor capacity lock", func(t *testing.T) {
		capacity := scope
		capacity.ActorID = "capacity-actor"
		newKey := func() dataservice.Credential {
			k := oldBinding
			k.ID = uuid.NewString()
			k.Scope = capacity
			k.State = "ACTIVE"
			k.Revision = 1
			k.CreatedAt = time.Now().UTC()
			return k
		}
		expired := newKey()
		_, _, err := store.Create(ctx, expired, uuid.NewString(), collection.Digest(expired.Input))
		require.NoError(t, err)
		past := expired.Input
		past.ExpiresAt = time.Now().UTC().Add(-time.Hour)
		raw, err := json.Marshal(past)
		require.NoError(t, err)
		require.NoError(t, db.Exec("UPDATE data_service_credentials SET config_json=?,expires_at=? WHERE id=?", string(raw), past.ExpiresAt, expired.ID).Error)
		for i := 0; i < 19; i++ {
			k := newKey()
			_, _, err := store.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
			require.NoError(t, err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			k := newKey()
			_, _, err := store.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
			results <- err
		}()
		go func() {
			<-start
			_, err := store.Change(ctx, capacity, expired.ID, uuid.NewString(), collection.Digest("extend expiry"), 1, dataservice.KeyPatch{State: "ACTIVE", Limits: &expired.Input})
			results <- err
		}()
		close(start)
		succeeded := 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				succeeded++
			} else {
				require.ErrorIs(t, err, dataservice.ErrConflict)
			}
		}
		require.Equal(t, 1, succeeded, "create and extension cannot both occupy the final slot")
		var count int64
		require.NoError(t, db.Raw("SELECT count(*) FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND state<>'REVOKED' AND expires_at>now()", capacity.OrganizationID, capacity.ActorID).Scan(&count).Error)
		require.Equal(t, int64(20), count)
	})
	t.Run("expired key cannot be extended when all twenty slots are occupied", func(t *testing.T) {
		capacity := scope
		capacity.ActorID = "full-capacity-actor"
		expired := oldBinding
		expired.ID, expired.Scope = uuid.NewString(), capacity
		_, _, err := store.Create(ctx, expired, uuid.NewString(), collection.Digest(expired.Input))
		require.NoError(t, err)
		past := expired.Input
		past.ExpiresAt = time.Now().UTC().Add(-time.Hour)
		raw, err := json.Marshal(past)
		require.NoError(t, err)
		require.NoError(t, db.Exec("UPDATE data_service_credentials SET config_json=?,expires_at=? WHERE id=?", string(raw), past.ExpiresAt, expired.ID).Error)
		for i := 0; i < 20; i++ {
			k := expired
			k.ID = uuid.NewString()
			_, _, err := store.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
			require.NoError(t, err)
		}
		_, err = store.Change(ctx, capacity, expired.ID, uuid.NewString(), collection.Digest("extend full"), 1, dataservice.KeyPatch{State: "ACTIVE", Limits: &expired.Input})
		require.ErrorIs(t, err, dataservice.ErrConflict)
		unchanged, err := store.Read(ctx, expired.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), unchanged.Revision)
		require.True(t, unchanged.Input.ExpiresAt.Before(time.Now()))
	})
	t.Run("revocation history does not hide usable credentials", func(t *testing.T) {
		history := scope
		history.ActorID = "history-actor"
		keeper := oldBinding
		keeper.ID = uuid.NewString()
		keeper.Scope = history
		keeper.CreatedAt = time.Now().UTC().Add(-24 * time.Hour)
		keeper.State = "ACTIVE"
		keeper.Revision = 1
		_, _, err := store.Create(ctx, keeper, uuid.NewString(), collection.Digest(keeper.Input))
		require.NoError(t, err)
		for i := 0; i < 101; i++ {
			k := keeper
			k.ID = uuid.NewString()
			k.CreatedAt = time.Now().UTC()
			_, _, err := store.Create(ctx, k, uuid.NewString(), collection.Digest(k.Input))
			require.NoError(t, err)
			_, err = store.Change(ctx, history, k.ID, uuid.NewString(), collection.Digest("revoke history"), 1, dataservice.KeyPatch{State: "REVOKED"})
			require.NoError(t, err)
		}
		keys, err := store.List(ctx, history)
		require.NoError(t, err)
		found := false
		for _, k := range keys {
			found = found || k.ID == keeper.ID
		}
		require.True(t, found, "the active key is still manageable")
		require.Len(t, keys, 1)
		page, err := store.History(ctx, history, "", 100)
		require.NoError(t, err)
		require.Len(t, page.Items, 100)
		require.NotEmpty(t, page.NextCursor)
		next, err := store.History(ctx, history, page.NextCursor, 100)
		require.NoError(t, err)
		require.Len(t, next.Items, 1)
		require.Empty(t, next.NextCursor)
		seen := map[string]bool{}
		for _, k := range append(page.Items, next.Items...) {
			require.False(t, seen[k.ID])
			seen[k.ID] = true
			require.Equal(t, "REVOKED", k.State)
		}
		foreign, err := store.History(ctx, collection.Scope{OrganizationID: "foreign", ActorID: history.ActorID, MemberID: history.MemberID}, "", 100)
		require.NoError(t, err)
		require.Empty(t, foreign.Items)
	})
	require.NoError(t, db.Exec("ALTER TABLE data_service_commands DROP CONSTRAINT data_service_commands_pkey; ALTER TABLE data_service_commands ADD PRIMARY KEY(organization_id,command_key)").Error)
	require.ErrorIs(t, VerifySchema(ctx, db), dataservice.ErrUnavailable, "wrong actor ownership key must fail readiness")
}
