package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue388Chain1AcceptanceAssemblyContract(t *testing.T) {
	repositoryRoot := filepath.Clean("..")
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(path)))
		require.NoError(t, err, "required #388 acceptance artifact %s", path)
		return string(raw)
	}

	harness := read("internal/app/httpapi/chain1_local_product_acceptance_test.go")
	orchestrationDriver := read("internal/imageagent/temporal/acceptancetest/driver.go")
	assembly := harness + orchestrationDriver
	for _, required := range []string{
		"productsourcing.NewInternalProducer",
		"NewProductReviewApplication",
		"NewImageAgentOrganizationApplication",
		"imageagenttemporal.NewOrganizationClient",
		"imageagenttemporal.NewWorker",
		"WorkerWireModeOrganization",
		"imageagentworker.OrganizationExecutionAuthorizer",
		"imageagenttools.NewProductImageSlotExecutor",
		"NewSheinRecordApplication",
	} {
		require.Contains(t, assembly, required, "#388 must compose the current owner %s", required)
	}
	for _, forbidden := range []string{
		"CommitApproval(",
		"INSERT INTO product_approved_assets",
		"INSERT INTO listing_shein_records",
		"INSERT INTO store_center_stores",
		"tenantbridge",
	} {
		require.NotContains(t, harness, forbidden, "#388 must not seed or reintroduce %s", forbidden)
	}

	script := read("scripts/chain1-local-product-acceptance.ps1")
	for _, required := range []string{
		"CHAIN1_ACCEPTANCE_DSN",
		"CHAIN1_TEMPORAL_ADDRESS",
		"TestChain1LocalProductAcceptance",
		"docker compose",
		"CHAIN1_STAGE",
	} {
		require.Contains(t, script, required)
	}
	require.NotContains(t, script, "ISSUE376_TEST_DSN")
	require.NotContains(t, script, "ISSUE382_TEST_DSN")

	compose := read("deployments/docker/chain1-acceptance/docker-compose.yml")
	require.Contains(t, compose, "chain1-postgres")
	require.Contains(t, compose, "chain1-temporal")
	require.Contains(t, compose, "CHAIN1_DB_PORT:-17443")
	require.Contains(t, compose, "CHAIN1_TEMPORAL_PORT:-17333")
	require.NotContains(t, strings.ToLower(compose), "zitadel")

	documentation := read("docs/development/chain1-local-product-acceptance.md")
	for _, required := range []string{"PASS", "FAIL", "SKIP", "NOT_RUN", "controlled external", "real PostgreSQL", "real Temporal"} {
		require.Contains(t, documentation, required)
	}
}
