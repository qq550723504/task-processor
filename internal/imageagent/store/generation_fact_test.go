package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/imageagent"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGenerationFactGormFencePreservesKnownSuccess(t *testing.T) {
	db := generationTestDatabase(t)
	base := NewGormRepository(db)
	initializeSlotEffectRun(t, base, "run-slot-effect-v3-generation")
	require.NoError(t, db.Model(&runRecord{}).Where("id = ?", "run-slot-effect-v3-generation").Updates(map[string]any{"scope_protocol": imageagent.OrganizationScopeProtocol, "member_id": "grant-1"}).Error)
	repository := NewOrganizationRepository(db).(*gormRepository)
	reservation := v3Reservation("generation")
	_, claimed, err := repository.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	intent := imageagent.GenerationIntent{Identity: reservation.Identity, MemberID: "grant-1", CatalogHash: "catalog-v1:" + strings.Repeat("a", 64), SourceDigest: strings.Repeat("b", 64), PromptVersion: "white-v1", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price-1", Points: 12, LimitVersion: 1, MonthStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	fact, err := repository.PrepareGenerationIntent(context.Background(), intent)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict, "intent must use this run's immutable catalog hash")
	catalog, err := repository.GetAssetCatalog(context.Background(), intent.Identity.RunScope)
	require.NoError(t, err)
	intent.CatalogHash = catalog.Manifest.Hash
	fact, err = repository.PrepareGenerationIntent(context.Background(), intent)
	require.NoError(t, err)
	changed := intent
	changed.PriceVersion = "price-2"
	_, err = repository.PrepareGenerationIntent(context.Background(), changed)
	require.Error(t, err)
	changed = intent
	changed.MemberID = "grant-2"
	_, err = repository.PrepareGenerationIntent(context.Background(), changed)
	require.Error(t, err)
	receipt := imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: intent.Identity.TenantID, MemberID: intent.MemberID, OperationID: "image-reserve:" + fact.IntentID, ReservationID: "reservation-1", ResourceType: "ai_point", Points: intent.Points, PriceVersion: intent.PriceVersion, LimitVersion: intent.LimitVersion, MonthStart: intent.MonthStart}
	_, err = repository.BindGenerationReservation(context.Background(), intent, receipt)
	require.NoError(t, err)
	var winners atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, won, e := repository.BeginGenerationDispatch(context.Background(), intent)
			if e == nil && won {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, winners.Load())
	_, won, err := repository.BeginGenerationDispatch(context.Background(), intent)
	require.NoError(t, err)
	require.False(t, won)
	_, err = repository.RecordGenerationNoEffect(context.Background(), intent)
	require.Error(t, err)
	_, err = repository.MarkGenerationUnknown(context.Background(), intent)
	require.NoError(t, err)
	proof := imageagent.GenerationSuccess{ResponseID: "response-1", ResultDigest: strings.Repeat("c", 64), ResultUnavailable: "invalid_result"}
	_, err = repository.RecordGenerationSuccess(context.Background(), intent, proof)
	require.NoError(t, err)
	_, err = repository.MarkGenerationUnknown(context.Background(), intent)
	require.NoError(t, err)
	proof.ResponseID = "other"
	_, err = repository.RecordGenerationSuccess(context.Background(), intent, proof)
	require.Error(t, err)
	// A new repository instance reads the same effect fact. Original V3 phase
	// writes must not erase known success or re-authorize provider dispatch.
	reopened := NewOrganizationRepository(db).(*gormRepository)
	got, err := reopened.ReadGenerationFact(context.Background(), intent.Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.GenerationSucceeded, got.State)
	require.Equal(t, "response-1", got.Success.ResponseID)
	_, won, err = reopened.BeginGenerationDispatch(context.Background(), intent)
	require.NoError(t, err)
	require.False(t, won)
}

func generationTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "image-agent.db")+"?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
