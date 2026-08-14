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

func TestGenerationUsageSettlementDependencyIsDisabledByDefault(t *testing.T) {
	t.Parallel()

	deps := buildListingKitCoreDependencies(buildListingKitServiceConfigInput{
		input:        BuildServiceInput{Config: &config.Config{}},
		repositories: &builtRepositories{subscriptionService: mustSubscriptionService(t)},
	})
	require.Nil(t, deps.GenerationUsageLedger)
}

func TestGenerationUsageSettlementDependencyWiresWhenEnabled(t *testing.T) {
	t.Parallel()

	deps := buildListingKitCoreDependencies(buildListingKitServiceConfigInput{
		input: BuildServiceInput{Config: &config.Config{ListingKit: config.ListingKitConfig{
			GenerationUsageLedgerEnabled: true,
		}}},
		repositories: &builtRepositories{subscriptionService: mustSubscriptionService(t)},
	})
	require.NotNil(t, deps.GenerationUsageLedger)
}

func mustSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	require.NoError(t, err)
	return svc
}
