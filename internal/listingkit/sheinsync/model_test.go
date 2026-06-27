package sheinsync

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

type sheinEnrollmentRunRepositoryStub struct{}

func (sheinEnrollmentRunRepositoryStub) CreateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (sheinEnrollmentRunRepositoryStub) UpdateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (sheinEnrollmentRunRepositoryStub) ListEnrollmentRuns(_ context.Context, _ *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error) {
	return nil, 0, nil
}

type sheinSyncRepositoryStub struct{}

func (sheinSyncRepositoryStub) UpsertSyncedProducts(_ context.Context, _ []*SheinSyncedProductRecord) error {
	return nil
}

func (sheinSyncRepositoryStub) ListSyncedProducts(_ context.Context, _ *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	return nil, 0, nil
}

func (sheinSyncRepositoryStub) UpdateManualCostPrice(_ context.Context, _ int64, _ *float64) error {
	return nil
}

func (sheinSyncRepositoryStub) MarkMissingSyncedProductsInactive(_ context.Context, _ int64, _ int64, _ []string) error {
	return nil
}

func (sheinSyncRepositoryStub) SaveSyncJob(_ context.Context, _ *SheinSyncJobRecord) error {
	return nil
}

func (sheinSyncRepositoryStub) ListSyncJobs(_ context.Context, _ *SheinSyncJobQuery) ([]SheinSyncJobRecord, int64, error) {
	return nil, 0, nil
}

func (sheinSyncRepositoryStub) SaveCandidates(_ context.Context, _ []*SheinActivityCandidateRecord) error {
	return nil
}

func (sheinSyncRepositoryStub) ListCandidates(_ context.Context, _ *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	return nil, 0, nil
}

func (sheinSyncRepositoryStub) CreateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (sheinSyncRepositoryStub) UpdateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (sheinSyncRepositoryStub) ListEnrollmentRuns(_ context.Context, _ *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error) {
	return nil, 0, nil
}

func (sheinSyncRepositoryStub) SaveEnrollmentItems(_ context.Context, _ []*SheinActivityEnrollmentItemRecord) error {
	return nil
}

func TestApplyEffectiveCostPrice(t *testing.T) {
	t.Parallel()

	t.Run("prefers manual cost price", func(t *testing.T) {
		t.Parallel()

		product := &SheinSyncedProductRecord{
			AutoCostPrice:   float64Ptr(12.5),
			ManualCostPrice: float64Ptr(15.8),
		}

		ApplyEffectiveCostPrice(product)

		require.NotNil(t, product.EffectiveCostPrice)
		require.Equal(t, 15.8, *product.EffectiveCostPrice)
		require.Equal(t, SheinCostPriceSourceManual, product.CostPriceSource)
	})

	t.Run("falls back to auto cost price", func(t *testing.T) {
		t.Parallel()

		product := &SheinSyncedProductRecord{
			AutoCostPrice: float64Ptr(12.5),
		}

		ApplyEffectiveCostPrice(product)

		require.NotNil(t, product.EffectiveCostPrice)
		require.Equal(t, 12.5, *product.EffectiveCostPrice)
		require.Equal(t, SheinCostPriceSourceAuto, product.CostPriceSource)
	})

	t.Run("clears effective cost price when missing sources", func(t *testing.T) {
		t.Parallel()

		product := &SheinSyncedProductRecord{
			EffectiveCostPrice: float64Ptr(99.9),
		}

		ApplyEffectiveCostPrice(product)

		require.Nil(t, product.EffectiveCostPrice)
		require.Equal(t, SheinCostPriceSourceNone, product.CostPriceSource)
	})
}

func TestSheinSyncRepositoryContract(t *testing.T) {
	t.Parallel()

	var _ SheinActivityEnrollmentRunRepository = sheinEnrollmentRunRepositoryStub{}
	var _ SheinSyncRepository = sheinSyncRepositoryStub{}
	require.NotNil(t, sheinSyncRepositoryStub{})
}

func TestSheinSyncedProductRecordUsesCompositeUniqueKey(t *testing.T) {
	t.Parallel()

	parsed, err := schema.Parse(&SheinSyncedProductRecord{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	uniqueKey := findIndexByName(parsed, "uk_listingkit_shein_synced_products_store_skc")

	require.NotNil(t, uniqueKey)
	require.True(t, uniqueKey.Class == "UNIQUE")
	require.Len(t, uniqueKey.Fields, 3)
	require.Equal(t, "TenantID", uniqueKey.Fields[0].Field.Name)
	require.Equal(t, "StoreID", uniqueKey.Fields[1].Field.Name)
	require.Equal(t, "SKCName", uniqueKey.Fields[2].Field.Name)
}

func TestSheinSyncedProductRecordCarriesBusinessModel(t *testing.T) {
	t.Parallel()

	parsed, err := schema.Parse(&SheinSyncedProductRecord{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	field := parsed.LookUpField("BusinessModel")
	require.NotNil(t, field)
	require.Equal(t, "business_model", field.DBName)
}

func TestSheinActivityCandidateRecordUsesTenantScopedUniqueKey(t *testing.T) {
	t.Parallel()

	parsed, err := schema.Parse(&SheinActivityCandidateRecord{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	uniqueKey := findIndexByName(parsed, "uk_listingkit_shein_activity_candidates_activity_skc_version")

	require.NotNil(t, uniqueKey)
	require.True(t, uniqueKey.Class == "UNIQUE")
	require.Len(t, uniqueKey.Fields, 6)
	require.Equal(t, "TenantID", uniqueKey.Fields[0].Field.Name)
	require.Equal(t, "StoreID", uniqueKey.Fields[1].Field.Name)
	require.Equal(t, "ActivityType", uniqueKey.Fields[2].Field.Name)
	require.Equal(t, "ActivityKey", uniqueKey.Fields[3].Field.Name)
	require.Equal(t, "SKCName", uniqueKey.Fields[4].Field.Name)
	require.Equal(t, "CandidateVersion", uniqueKey.Fields[5].Field.Name)
}

func TestSheinActivityEnrollmentItemRecordSupportsIdempotencyIdentity(t *testing.T) {
	t.Parallel()

	parsed, err := schema.Parse(&SheinActivityEnrollmentItemRecord{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	runIDField := parsed.LookUpField("RunID")
	require.NotNil(t, runIDField)

	candidateIDField := parsed.LookUpField("CandidateID")
	require.NotNil(t, candidateIDField)

	uniqueKey := findIndexByName(parsed, "uk_listingkit_shein_enrollment_items_run_candidate")

	require.NotNil(t, uniqueKey)
	require.True(t, uniqueKey.Class == "UNIQUE")
	require.Len(t, uniqueKey.Fields, 2)
	require.Equal(t, "RunID", uniqueKey.Fields[0].Field.Name)
	require.Equal(t, "CandidateID", uniqueKey.Fields[1].Field.Name)
}

func findIndexByName(parsed *schema.Schema, name string) *schema.Index {
	for _, index := range parsed.ParseIndexes() {
		if index.Name == name {
			return index
		}
	}
	return nil
}

func float64Ptr(v float64) *float64 {
	return &v
}
