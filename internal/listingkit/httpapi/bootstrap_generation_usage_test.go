package httpapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/listingsubscription"
)

func TestBuildGenerationUsageLedgerSupportsKnownRepositories(t *testing.T) {
	t.Parallel()

	mem, err := buildGenerationUsageLedger(listingsubscription.NewMemRepository())
	require.NoError(t, err)
	require.NotNil(t, mem)
}

func TestBuildGenerationUsageLedgerRejectsUnknownRepository(t *testing.T) {
	t.Parallel()

	var repo listingsubscription.Repository
	_, err := buildGenerationUsageLedger(repo)
	require.Error(t, err)
}

func TestGenerationUsageSettlementDependencyKeepsLedgerForRecoveryWhenAdmissionDisabled(t *testing.T) {
	t.Parallel()

	deps := buildListingKitCoreDependencies(buildListingKitServiceConfigInput{
		input:        BuildServiceInput{Config: &config.Config{}},
		repositories: &builtRepositories{subscriptionService: mustUsageLedgerSubscriptionService(t)},
	})
	require.NotNil(t, deps.GenerationUsageLedger)
	require.NotNil(t, deps.GenerationUsageAdmission)
	require.False(t, deps.GenerationUsageAdmission.AllowsGenerationUsage("tenant-17"))
}

func TestGenerationUsageSettlementDependencyLimitsAdmissionToConfiguredCohort(t *testing.T) {
	t.Parallel()

	deps := buildListingKitCoreDependencies(buildListingKitServiceConfigInput{
		input: BuildServiceInput{Config: &config.Config{ListingKit: config.ListingKitConfig{
			GenerationUsageLedgerEnabled:   true,
			GenerationUsageLedgerTenantIDs: []string{"tenant-17", "  tenant-18  "},
		}}},
		repositories: &builtRepositories{subscriptionService: mustUsageLedgerSubscriptionService(t)},
	})
	require.NotNil(t, deps.GenerationUsageLedger)
	require.True(t, deps.GenerationUsageAdmission.AllowsGenerationUsage("tenant-17"))
	require.True(t, deps.GenerationUsageAdmission.AllowsGenerationUsage("tenant-18"))
	require.False(t, deps.GenerationUsageAdmission.AllowsGenerationUsage("tenant-19"))
}

func mustSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	require.NoError(t, err)
	return svc
}

func mustUsageLedgerSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	require.NoError(t, err)
	return svc
}
