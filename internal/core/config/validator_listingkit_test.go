package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfigRejectsLegacyListingKitUsernameAllowlistWithoutExposingValues(t *testing.T) {
	const configuredValue = "sensitive-legacy-subject"
	cfg := NewDefaultConfig()
	cfg.OpenAI.APIKey = "test-openai-key"
	cfg.ListingKit.Zitadel.AllowedUsernames = []string{configuredValue}

	err := cfg.ValidateWithError()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listingkit.zitadel.allowedUsernames")
	assert.Contains(t, err.Error(), "obsolete")
	assert.NotContains(t, err.Error(), configuredValue)
}

func TestListingKitLegacyUsernameAllowlistInputsFailClosed(t *testing.T) {
	const configuredValue = "sensitive-legacy-subject"
	baseConfig := strings.Join([]string{
		"openai:",
		"  apiKey: test-openai-key",
		"  model: gemini-2.5-flash",
		"  baseURL: https://api.example.test/v1",
	}, "\n")

	t.Run("yaml", func(t *testing.T) {
		clearLegacyListingKitUsernameAllowlistEnv(t)
		configPath := filepath.Join(t.TempDir(), "config-test.yaml")
		require.NoError(t, os.WriteFile(
			configPath,
			[]byte(baseConfig+"\nlistingkit:\n  zitadel:\n    allowedUsernames: [\""+configuredValue+"\"]\n"),
			0o600,
		))

		_, err := LoadConfigFromFile(configPath)
		requireLegacyUsernameAllowlistRejection(t, err, configuredValue)
	})

	for _, envName := range []string{
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
		"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
	} {
		t.Run(envName, func(t *testing.T) {
			clearLegacyListingKitUsernameAllowlistEnv(t)
			t.Setenv(envName, configuredValue)

			_, err := LoadFromBytes([]byte(baseConfig))
			requireLegacyUsernameAllowlistRejection(t, err, configuredValue)
		})
	}

	t.Run("yaml is not shadowed by blank primary env", func(t *testing.T) {
		clearLegacyListingKitUsernameAllowlistEnv(t)
		t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "   ")
		configPath := filepath.Join(t.TempDir(), "config-test.yaml")
		require.NoError(t, os.WriteFile(
			configPath,
			[]byte(baseConfig+"\nlistingkit:\n  zitadel:\n    allowedUsernames: [\""+configuredValue+"\"]\n"),
			0o600,
		))

		_, err := LoadConfigFromFile(configPath)
		requireLegacyUsernameAllowlistRejection(t, err, configuredValue)
	})

	t.Run("deprecated env is not shadowed by blank primary env", func(t *testing.T) {
		clearLegacyListingKitUsernameAllowlistEnv(t)
		t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "   ")
		t.Setenv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", configuredValue)

		_, err := LoadFromBytes([]byte(baseConfig))
		requireLegacyUsernameAllowlistRejection(t, err, configuredValue)
	})
}

func clearLegacyListingKitUsernameAllowlistEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
		"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
	} {
		value, existed := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if existed {
				require.NoError(t, os.Setenv(name, value))
				return
			}
			require.NoError(t, os.Unsetenv(name))
		})
	}
}

func requireLegacyUsernameAllowlistRejection(t *testing.T, err error, configuredValue string) {
	t.Helper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listingkit.zitadel.allowedUsernames")
	assert.Contains(t, err.Error(), "obsolete")
	assert.NotContains(t, err.Error(), configuredValue)
}
