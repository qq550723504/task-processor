package currentapplication

import (
	"context"
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"task-processor/internal/storecenter"
	"testing"
)

func TestOfficialStoreConfigRequiresSeparatePrivateKeysAndFixedHTTPSOrigin(t *testing.T) {
	c := &OfficialStoreConnectionConfig{AppID: "test-app", Version: "config-1", Type: storecenter.ApplicationSelfOperated, APIOrigin: "https://openapi.sheincorp.com", CallbackURL: "https://localhost:22744/workbench/stores/shein/callback", AppSecretFile: filepath.Join(t.TempDir(), "app-secret"), CredentialKeyFile: filepath.Join(t.TempDir(), "merchant-key"), CredentialKeyID: "merchant-key-1"}
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
	missing.StoreCenter.OfficialApplications = []OfficialStoreConnectionConfig{*c}
	c.AppSecretFile = "relative-secret"
	missing.StoreCenter.OfficialApplications = []OfficialStoreConnectionConfig{*c}
	require.Error(t, missing.validate())
	var unset *OfficialStoreConnectionConfig
	provider, protection, err = unset.prepare(context.Background())
	require.NoError(t, err)
	require.Nil(t, provider)
	require.Nil(t, protection)
}
func TestOfficialApplicationsRejectSharedFilesKeysAndUnknownTypes(t *testing.T) {
	configs := []OfficialStoreConnectionConfig{}
	for _, mode := range []storecenter.OfficialApplicationType{storecenter.ApplicationSelfOperated, storecenter.ApplicationSemiManaged, storecenter.ApplicationFullyManaged} {
		root := t.TempDir()
		configs = append(configs, OfficialStoreConnectionConfig{AppID: string(mode), Version: "v1", Type: mode, APIOrigin: "https://openapi.sheincorp.com", CallbackURL: "https://localhost/callback", AppSecretFile: filepath.Join(root, "app-secret"), CredentialKeyFile: filepath.Join(root, "key"), CredentialKeyID: string(mode)})
	}
	require.NoError(t, validateOfficialApplications(configs))
	for _, mutate := range []func([]OfficialStoreConnectionConfig){func(c []OfficialStoreConnectionConfig) { c[1].AppID = c[0].AppID }, func(c []OfficialStoreConnectionConfig) { c[1].CredentialKeyID = c[0].CredentialKeyID }, func(c []OfficialStoreConnectionConfig) { c[1].AppSecretFile = c[0].CredentialKeyFile }, func(c []OfficialStoreConnectionConfig) { c[1].Type = "unknown" }} {
		copy := append([]OfficialStoreConnectionConfig(nil), configs...)
		mutate(copy)
		require.Error(t, validateOfficialApplications(copy))
	}
}
