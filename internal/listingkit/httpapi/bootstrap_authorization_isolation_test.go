package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestPrepareModuleServiceEnvironmentDoesNotConfigureRouteAuthorizationSingletons(t *testing.T) {
	const sentinelIssuer = "https://sentinel.example"
	restore := SetListingKitZitadelAuthConfigForTesting(&listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{IssuerURL: sentinelIssuer},
	})
	t.Cleanup(restore)
	input := BuildServiceInput{
		Config: &config.Config{},
		Logger: logrus.New(),
		Hooks: BuildServiceHooks{
			LegacyTenantResolverConfigurator: func(*config.Config, *logrus.Logger) (func() error, error) {
				return nil, nil
			},
		},
	}

	require.NoError(t, prepareModuleServiceEnvironment(input, &closerStack{}))
	after := currentListingKitZitadelRuntimeConfig()
	require.NotNil(t, after)
	require.Equal(t, sentinelIssuer, after.AuthConfig.IssuerURL,
		"module construction must not overwrite route authorization singleton state")
}
