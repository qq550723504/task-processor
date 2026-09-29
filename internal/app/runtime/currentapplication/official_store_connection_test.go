package currentapplication

import (
	"context"
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialStoreConfigRequiresSeparatePrivateKeysAndFixedHTTPSOrigin(t *testing.T) {
	c := &OfficialStoreConnectionConfig{AppID: "test-app", Version: "config-1", APIOrigin: "https://openapi.sheincorp.com", CallbackURL: "https://localhost:22744/workbench/stores/shein/callback", AppSecretFile: filepath.Join(t.TempDir(), "app-secret"), CredentialKeyFile: filepath.Join(t.TempDir(), "merchant-key"), CredentialKeyID: "merchant-key-1"}
	require.NoError(t, c.validate())
	require.NoError(t, os.WriteFile(c.AppSecretFile, []byte(strings.Repeat("synthetic", 4)), 0600))
	require.NoError(t, os.WriteFile(c.CredentialKeyFile, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0600))
	privatizeSyntheticTestFile(t, c.AppSecretFile)
	privatizeSyntheticTestFile(t, c.CredentialKeyFile)
	provider, protection, err := c.prepare(context.Background())
	require.NoError(t, err)
	require.NotNil(t, provider)
	require.NotNil(t, protection)
	for _, mutate := range []func(*OfficialStoreConnectionConfig){func(c *OfficialStoreConnectionConfig) { c.APIOrigin = "https://untrusted.example" }, func(c *OfficialStoreConnectionConfig) { c.CallbackURL = "http://localhost/callback" }, func(c *OfficialStoreConnectionConfig) { c.CallbackURL += "?org=other" }, func(c *OfficialStoreConnectionConfig) { c.CredentialKeyFile = c.AppSecretFile }, func(c *OfficialStoreConnectionConfig) { c.Version = "" }} {
		copy := *c
		mutate(&copy)
		require.Error(t, copy.validate())
	}
	require.NoError(t, os.WriteFile(c.CredentialKeyFile, []byte("not-a-key"), 0600))
	_, _, err = c.prepare(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "not-a-key")
	missing := storeRuntimeConfig()
	missing.StoreCenter.OfficialConnection = c
	c.AppSecretFile = "relative-secret"
	require.Error(t, missing.validate())
	var unset *OfficialStoreConnectionConfig
	provider, protection, err = unset.prepare(context.Background())
	require.NoError(t, err)
	require.Nil(t, provider)
	require.Nil(t, protection)
}
