package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromBytes_HonorsListingKitOwnerScopeRequired(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(strings.Join([]string{
		"openai:",
		"  apiKey: \"test-openai-key\"",
		"  model: \"gemini-2.5-flash\"",
		"  baseURL: \"https://api.example.test/v1\"",
		"  timeout: 30",
		"listingkit:",
		"  ownerScopeRequired: false",
	}, "\n")))
	require.NoError(t, err)

	assert.False(t, cfg.ListingKit.OwnerScopeRequired)
}

func TestNewViper_BindsListingKitOwnerScopeEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_OWNER_SCOPE_REQUIRED", "true")

	v := newViper()

	assert.True(t, v.GetBool("listingkit.ownerScopeRequired"))
}

func TestNewViper_BindsDeprecatedListingKitZitadelOwnerScopeEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED", "true")

	v := newViper()

	assert.True(t, v.GetBool("listingkit.ownerScopeRequired"))
}
