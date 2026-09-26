package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/ledger/orgresource"
)

type generationPointTestAuth struct{}

func (generationPointTestAuth) AuthorizeImageGeneration(context.Context, imageagent.GenerationIntent) error {
	return nil
}

type heldGenerationRead struct {
	owner  *gormRepository
	mu     sync.Mutex
	first  bool
	read   chan struct{}
	resume chan struct{}
}

func (h *heldGenerationRead) ReadGenerationFact(ctx context.Context, id imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error) {
	fact, err := h.owner.ReadGenerationFact(ctx, id)
	h.mu.Lock()
	first := !h.first
	h.first = true
	h.mu.Unlock()
	if first {
		close(h.read)
		select {
		case <-h.resume:
		case <-ctx.Done():
			return imageagent.GenerationFact{}, ctx.Err()
		}
	}
	return fact, err
}

func generationPointDatabases(t *testing.T) (*gormRepository, *gorm.DB, *resourceadapter.GormMemberLimitRepository, imageagent.GenerationIntent) {
	t.Helper()
	ctx := context.Background()
	imageDB := generationTestDatabase(t)
	initializeSlotEffectRun(t, NewGormRepository(imageDB), "run-slot-effect-v3-points")
	require.NoError(t, imageDB.Model(&runRecord{}).Where("id = ?", "run-slot-effect-v3-points").Updates(map[string]any{"scope_protocol": imageagent.OrganizationScopeProtocol, "member_id": "grant-1"}).Error)
	owner := NewOrganizationRepository(imageDB).(*gormRepository)
	reservation := v3Reservation("points")
	_, won, err := owner.ReserveSlotProviderV3(ctx, reservation)
	require.NoError(t, err)
	require.True(t, won)
	catalog, err := owner.GetAssetCatalog(ctx, reservation.Identity.RunScope)
	require.NoError(t, err)
	intent := imageagent.GenerationIntent{Identity: reservation.Identity, MemberID: "grant-1", CatalogHash: catalog.Manifest.Hash, SourceDigest: strings.Repeat("b", 64), PromptVersion: "prompt-1", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price-1", Points: 12, LimitVersion: 1, MonthStart: orgresource.AIPointMonthStart(time.Now())}
	_, err = owner.PrepareGenerationIntent(ctx, intent)
	require.NoError(t, err)
	// A genuinely separate database, not two connections to the same owner DB.
	commercial, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "commercial.db")+"?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := commercial.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, resourceadapter.AutoMigrate(commercial))
	require.NoError(t, commercial.Exec("INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,reserved,consumed,created_at,updated_at) VALUES (?, ?, ?, 0, 0, ?, ?)", intent.Identity.TenantID, "ai_point", 100, time.Now(), time.Now()).Error)
	limits, err := resourceadapter.NewGormMemberLimitRepository(commercial, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: intent.Identity.TenantID, MemberID: intent.MemberID, ActorID: "admin-1", OperationID: "limit-1", Target: 20})
	require.NoError(t, err)
	return owner, commercial, limits, intent
}

func TestGenerationPointsLateReserveCannotOutrunDurableNoGeneration(t *testing.T) {
	owner, commercial, _, intent := generationPointDatabases(t)
	held := &heldGenerationRead{owner: owner, read: make(chan struct{}), resume: make(chan struct{})}
	r, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, held, generationPointTestAuth{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := r.ReserveImageGeneration(ctx, intent.Identity); done <- err }()
	select {
	case <-held.read:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	_, err = owner.RecordGenerationNoEffect(ctx, intent)
	require.NoError(t, err)
	closed, err := r.FinalizeImageGeneration(ctx, intent.Identity)
	require.NoError(t, err)
	require.Equal(t, "no_reservation", closed.State)
	_, err = owner.BindGenerationSettlement(ctx, intent, closed)
	require.NoError(t, err)
	close(held.resume)
	require.ErrorIs(t, <-done, orgresource.ErrOwnerNotReservable)
	var count int64
	require.NoError(t, commercial.Table("saas_organization_resource_reservations").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, commercial.Table("saas_organization_resource_events").Count(&count).Error)
	require.Zero(t, count, "closing a not-yet-reserved operation is not a zero-value ledger event")
	_, won, err := owner.BeginGenerationDispatch(ctx, intent)
	require.NoError(t, err)
	require.False(t, won)
	readback, err := owner.ReadGenerationFact(ctx, intent.Identity)
	require.NoError(t, err)
	require.Equal(t, closed, readback.Settlement)
}

func TestGenerationPointsConfirmedNoEffectReleasesExistingHoldWithoutBinding(t *testing.T) {
	owner, commercial, limits, intent := generationPointDatabases(t)
	r, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
	require.NoError(t, err)
	ctx := context.Background()
	receipt, err := r.ReserveImageGeneration(ctx, intent.Identity)
	require.NoError(t, err)
	// Simulate the resource ACK/binding being lost, then the V3 recovery CAS
	// proving this attempt never obtained a dispatch permission.
	_, err = owner.RecordGenerationNoEffect(ctx, intent)
	require.NoError(t, err)
	settled, err := r.FinalizeImageGeneration(ctx, intent.Identity)
	require.NoError(t, err)
	require.Equal(t, "released", settled.State)
	require.Equal(t, receipt.ReservationID, settled.ReservationID)
	_, err = owner.BindGenerationSettlement(ctx, intent, settled)
	require.NoError(t, err)
	again, err := r.FinalizeImageGeneration(ctx, intent.Identity)
	require.NoError(t, err)
	require.Equal(t, settled, again)
	read, err := limits.ReadMonthlyLimit(ctx, intent.Identity.TenantID, intent.MemberID)
	require.NoError(t, err)
	require.Zero(t, read.Reserved)
	require.Zero(t, read.Consumed)
	var bucket struct{ Available, Reserved, Consumed int64 }
	require.NoError(t, commercial.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", intent.Identity.TenantID, "ai_point").Take(&bucket).Error)
	require.EqualValues(t, 100, bucket.Available)
	require.Zero(t, bucket.Reserved)
	require.Zero(t, bucket.Consumed)
}

func TestGenerationPointsReserveAndLowerLimitUseSameMemberFence(t *testing.T) {
	owner, commercial, limits, intent := generationPointDatabases(t)
	r, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
	require.NoError(t, err)
	ctx := context.Background()
	start := make(chan struct{})
	reserveErr := make(chan error, 1)
	limitErr := make(chan error, 1)
	go func() { <-start; _, err := r.ReserveImageGeneration(ctx, intent.Identity); reserveErr <- err }()
	go func() {
		<-start
		_, err := limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: intent.Identity.TenantID, MemberID: intent.MemberID, ActorID: "admin-1", OperationID: "lower-1", ExpectedVersion: 1, Target: 5})
		limitErr <- err
	}()
	close(start)
	reserveResult, limitResult := <-reserveErr, <-limitErr
	require.True(t, (reserveResult == nil) != (limitResult == nil), "exactly one may win: reserve=%v limit=%v", reserveResult, limitResult)
	read, err := limits.ReadMonthlyLimit(ctx, intent.Identity.TenantID, intent.MemberID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, read.MonthlyLimit, read.Reserved+read.Consumed)
	if reserveResult == nil {
		require.EqualValues(t, 12, read.Reserved)
		require.ErrorIs(t, limitResult, orgresource.ErrMemberLimitExceeded)
	} else {
		require.EqualValues(t, 5, read.MonthlyLimit)
		require.Zero(t, read.Reserved)
		require.ErrorIs(t, reserveResult, orgresource.ErrMemberLimitVersionConflict)
	}
	// Neither outcome makes an implicit provider dispatch decision.
	fact, err := owner.ReadGenerationFact(ctx, intent.Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.GenerationPrepared, fact.State)
}
