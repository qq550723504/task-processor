package currentapplication

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrivateDraftTrialAdmission(t *testing.T) {
	makeConfig := func() *Config {
		c := acquisitionRuntimeConfig()
		c.Identity.IssuerURL = "https://localhost:18080"
		c.Identity.AuthorizationAPIURL = c.Identity.IssuerURL
		c.Identity.TenantDirectoryToken = "fixture"
		c.ProductCollections = true
		c.StoreCenter = &StoreCenterConfig{Enabled: true, Database: DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "store_center_runtime", Password: "fixture", Database: "stores", MaxConnections: 2}}
		c.AgentCustomizationDatabase = &DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "agent_customization_runtime", Password: "fixture", Database: "customization", MaxConnections: 2}
		c.PrivateDraftTrial = &PrivateDraftTrialConfig{Acknowledgment: "ISOLATED_OFFLINE_DRAFT_TRIAL_ONLY", OrganizationID: "test-org", ActorID: "test-actor", MemberID: "test-member", StoreID: "11111111-1111-4111-8111-111111111111"}
		return c
	}
	require.NoError(t, makeConfig().validate())
	for _, pod := range []bool{false, true} {
		c := makeConfig()
		c.SupplyMarket = &SupplyMarketConfig{Storage: KnowledgeStorageConfig{Region: "local", Bucket: "private", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws"}}
		if pod {
			c.POD = &PODConfig{}
		}
		require.Error(t, c.validatePrivateDraftTrial(), "offline trial excludes market and POD services")
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.PrivateDraftTrial.Acknowledgment = "" },
		func(c *Config) { c.PrivateDraftTrial.MemberID = "" },
		func(c *Config) { c.PrivateDraftTrial.StoreID = "wrong" },
		func(c *Config) { c.Identity.IssuerURL = "http://localhost:18080" },
		func(c *Config) { c.SupplyChain = &SupplyChainConfig{} },
		func(c *Config) { c.LocalTrial = &LocalTrialConfig{} },
		func(c *Config) { c.ProductCollections = false },
		func(c *Config) { c.AgentCustomizationDatabase = nil },
		func(c *Config) { c.ProductAgent = &ProductAgentConfig{} },
		func(c *Config) { c.StoreCenter.OfficialApplications = []OfficialStoreConnectionConfig{{}} },
	} {
		c := makeConfig()
		mutate(c)
		require.Error(t, c.validate())
	}
}
