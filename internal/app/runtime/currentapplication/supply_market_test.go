package currentapplication

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSupplyMarketConfigurationRequiresCurrentOwners(t *testing.T) {
	c := supplyRuntimeConfig(t)
	c.SupplyMarket = &SupplyMarketConfig{Storage: KnowledgeStorageConfig{Region: "local", Bucket: "private-qualifications", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws", Endpoint: "http://127.0.0.1:9000"}}
	require.NoError(t, c.validateSupplyMarket())
	c.ProductCollections = false
	require.Error(t, c.validateSupplyMarket())
	c.ProductCollections = true
	c.SupplyMarket.Storage.Mode = ""
	require.Error(t, c.validateSupplyMarket())
}

func TestPODAssetsStaySeparateFromAgentCustomization(t *testing.T) {
	c := supplyRuntimeConfig(t)
	c.SupplyMarket = &SupplyMarketConfig{Storage: KnowledgeStorageConfig{Region: "local", Bucket: "private", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws"}}
	c.POD = &PODConfig{AssetDatabase: c.SupplyChain.AssetDatabase, TemporalAddress: c.SupplyChain.TemporalAddress, TemporalNamespace: "default", CredentialFile: writeManifest(t, `{}`), OSSHosts: []string{"fixture.oss-cn-hangzhou.aliyuncs.com"}}
	c.AgentCustomizationDatabase = &DatabaseConfig{Host: c.POD.AssetDatabase.Host, Port: c.POD.AssetDatabase.Port, Database: c.POD.AssetDatabase.Database, User: "agent_customization_runtime", Password: "fixture", MaxConnections: 2}
	require.Error(t, c.validateSupplyMarket(), "POD cannot open another current owner's database")
}

func TestPODAssetsStaySeparateFromReportCenter(t *testing.T) {
	c := supplyRuntimeConfig(t)
	c.SupplyMarket = &SupplyMarketConfig{Storage: KnowledgeStorageConfig{Region: "local", Bucket: "private", AccessKeyID: "fixture", SecretAccessKey: "fixture", Mode: "aws"}}
	c.POD = &PODConfig{AssetDatabase: c.SupplyChain.AssetDatabase, TemporalAddress: c.SupplyChain.TemporalAddress, TemporalNamespace: "default", CredentialFile: writeManifest(t, `{}`), OSSHosts: []string{"fixture.oss-cn-hangzhou.aliyuncs.com"}}
	c.ReportCenter = &ReportCenterConfig{Database: c.POD.AssetDatabase}
	c.ReportCenter.Database.User = "report_center_runtime"
	require.Error(t, c.validateSupplyMarket(), "POD cannot open the report owner's database")
}
