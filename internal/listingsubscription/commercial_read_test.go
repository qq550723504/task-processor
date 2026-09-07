package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

func commercialIdentity(org, role string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		UserID: "commercial-reader", HomeOrganizationID: "home-A", TenantID: org,
		EffectiveOrganizationID: org, Roles: []string{role}, TokenExpiresAt: time.Now().Add(time.Hour),
	})
}

func TestCommercialReadActualGrantAndLedgerWindow(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	svc, err := NewService(repo)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = svc.ApplyPlan(ctx, "org-B", PlanApplyInput{PlanCode: PlanProfessional, Status: StatusActive}, "fixture-admin")
	require.NoError(t, err)
	_, err = svc.UpsertEntitlement(ctx, "org-B", ModuleListingKit, EntitlementInput{Status: StatusActive, Limits: map[string]int{"listingkit_generations_succeeded": 0}})
	require.NoError(t, err)
	ledger := NewGormUsageLedger(repo)
	now := time.Now().UTC()
	for _, month := range []time.Time{now.AddDate(0, -1, 0), now} {
		input := ReserveUsageInput{TenantID: "org-B", ModuleCode: ModuleListingKit, Metric: usageMetricListingKitGenerationsSucceeded, Quantity: 1, PeriodKey: month.Format("2006-01"), SourceType: "listingkit_generation", SourceID: month.Format("2006-01"), IdempotencyKey: month.Format("2006-01"), OccurredAt: month}
		reserved, reserveErr := ledger.Reserve(ctx, input)
		require.NoError(t, reserveErr)
		_, commitErr := ledger.Commit(ctx, reserved.Event.EventID)
		require.NoError(t, commitErr)
	}
	reader := NewCommercialReadService(repo, authz.DefaultListingKitAuthorizer())
	got, err := reader.Read(commercialIdentity("org-B", "listingkit_operator"))
	require.NoError(t, err)
	require.Equal(t, "org-B", got.OrganizationID)
	require.Equal(t, PlanProfessional, got.Subscription.PlanCode)
	require.Len(t, got.Plans, 1, "technical default plans are not the sellable catalog")
	require.Equal(t, "base_payg", got.Plans[0].Code)
	require.Len(t, got.Usage, 5)
	require.Equal(t, "1", *got.Usage[0].Committed, "only the current month is read")
	require.Equal(t, "0", *got.Usage[0].Reserved)
	require.Equal(t, "unknown", got.Usage[1].State)
	require.Nil(t, got.Usage[1].Committed)
	require.Equal(t, "unsupported", got.ResourceBalance.State)
	var found bool
	for _, entitlement := range got.Entitlements {
		if entitlement.ModuleCode == ModuleListingKit {
			found = true
			require.Len(t, entitlement.Limits, 1)
			require.Equal(t, "unlimited", entitlement.Limits[0].Kind)
			require.Equal(t, "0", entitlement.Limits[0].RawValue)
			require.Nil(t, entitlement.Limits[0].Value)
		}
	}
	require.True(t, found)
	other, err := reader.Read(commercialIdentity("org-empty", "listingkit_admin"))
	require.NoError(t, err)
	require.Nil(t, other.Subscription)
	require.Empty(t, other.Entitlements)
	for _, usage := range other.Usage {
		require.Nil(t, usage.Committed)
	}
	encoded, err := json.Marshal(other)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"subscription":null`)
	require.Contains(t, string(encoded), `"entitlements":[]`)
}

func TestCommercialReadDoesNotSynthesizeMissingOSSOrWriteCatalog(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	_, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: "org-B", ModuleCode: ModuleListingKit, Status: StatusActive})
	require.NoError(t, err)
	reader := NewCommercialReadService(repo, authz.DefaultListingKitAuthorizer())
	got, err := reader.Read(commercialIdentity("org-B", "listingkit_admin"))
	require.NoError(t, err)
	require.Len(t, got.Entitlements, 1)
	require.Equal(t, ModuleListingKit, got.Entitlements[0].ModuleCode)
	for _, table := range []string{"saas_plans", "saas_modules", "saas_tenant_subscriptions", "saas_usage_events", "saas_usage_buckets", "saas_subscription_audit_logs"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
}

type commercialSpy struct{ calls int }

func (s *commercialSpy) ReadCommercialOverview(context.Context, string, time.Time) (*CommercialOverview, error) {
	s.calls++
	return nil, errors.New("private database failure")
}

func TestCommercialReadAuthorizationPrecedesPersistence(t *testing.T) {
	spy := &commercialSpy{}
	reader := NewCommercialReadService(spy, authz.DefaultListingKitAuthorizer())
	for _, ctx := range []context.Context{context.Background(), commercialIdentity("org-B", "listingkit_viewer"), commercialIdentity("", "listingkit_admin")} {
		_, err := reader.Read(ctx)
		require.Error(t, err)
	}
	require.Zero(t, spy.calls)
	_, err := reader.Read(commercialIdentity("org-B", "listingkit_operator"))
	require.ErrorIs(t, err, ErrCommercialUnavailable)
	require.NotContains(t, err.Error(), "private")
	require.Equal(t, 1, spy.calls)
}

func TestCommercialReadLimitsRespectCanonicalKeysAndUnknown(t *testing.T) {
	limits, unknown, err := commercialLimits(ModuleListingKit, `{"product_image_jobs_succeeded":0,"product_image_jobs":100,"unrecognized":17}`)
	require.NoError(t, err)
	require.Equal(t, 1, unknown)
	require.Len(t, limits, 1)
	require.Equal(t, "product_image_jobs_succeeded", limits[0].SourceKey)
	require.Equal(t, "operation", limits[0].Unit)
	require.Equal(t, "unlimited", limits[0].Kind)
	limits, unknown, err = commercialLimits(ModuleStoreManagement, `{"store_count":0}`)
	require.NoError(t, err)
	require.Zero(t, unknown)
	require.Equal(t, "finite", limits[0].Kind)
	require.Equal(t, "0", *limits[0].Value)
	limits, _, err = commercialLimits(ModuleListingKit, `{}`)
	require.NoError(t, err)
	require.Empty(t, limits)
	for _, raw := range []string{`{"product_image_jobs":-1}`, `{"storage_bytes_current":1.5}`, `{"bad":9223372036854775808}`, `not-json`} {
		_, _, err = commercialLimits(ModuleListingKit, raw)
		require.Error(t, err, raw)
	}
}

func TestCommercialReadInvalidPersistenceDoesNotBecomeEmpty(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	_, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: "org-B", ModuleCode: ModuleListingKit, Status: StatusActive})
	require.NoError(t, err)
	require.NoError(t, db.Model(&tenantEntitlementRow{}).Where("tenant_id = ?", "org-B").Update("limits", `{"bad":-1}`).Error)
	_, err = NewCommercialReadService(repo, authz.DefaultListingKitAuthorizer()).Read(commercialIdentity("org-B", "listingkit_admin"))
	require.ErrorIs(t, err, ErrCommercialUnavailable)
}
