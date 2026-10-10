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
