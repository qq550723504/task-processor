package bootstrap

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestBuildSharedResources_AllowsMissingManagementAuth(t *testing.T) {
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

	resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{
		AllowMissingManagementAuth: true,
	})
	require.NoError(t, err)
	require.NotNil(t, resources)
	require.Nil(t, resources.AuthClient)
	require.Nil(t, resources.ManagementClient)
}
