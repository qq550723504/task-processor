package resources

import (
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestBuildSharedResourcesDoesNotConstructRetiredManagementService(t *testing.T) {
	content, err := os.ReadFile("shared_resources.go")
	require.NoError(t, err)

	for _, token := range []string{
		`"task-processor/internal/infra/clients/management"`,
		"ManagementClient        *management.ClientManager",
		"management.NewClientManager",
		"newConfiguredManagementClient",
		"AllowMissingManagementAuth",
		"SkipManagementAuth",
	} {
		require.NotContains(t, string(content), token)
	}
	require.False(t, strings.Contains(string(content), "managementClient:"))
}

func TestBuildSharedResourcesDoesNotConfigureRetiredManagementAuth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Management: config.ManagementConfig{
			BaseURL:      "http://127.0.0.1:1",
			ClientID:     "test-client",
			ClientSecret: "bad-secret",
		},
		Amazon: config.AmazonConfig{
			DataFreshnessDays: 15,
		},
	}

	resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{})
	require.NoError(t, err)
	require.NotNil(t, resources)
	require.Nil(t, resources.AuthClient)
	require.Nil(t, resources.ProcessorRuntime)
	require.Nil(t, resources.ListingRuntimeHealthValidator)
}
