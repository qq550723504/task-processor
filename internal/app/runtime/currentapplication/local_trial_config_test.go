package currentapplication

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue36LocalTrialRequiresHTTPSAndNoProviderExecution(t *testing.T) {
	baseline := storeRuntimeConfig()
	baseline.StoreCenter.Database.Port = 5433
	baseline.StoreCenter.Database.Database = "store_center"
	trial := &LocalTrialConfig{Enabled: true, Database: DatabaseConfig{
		Host: baseline.StoreCenter.Database.Host, Port: baseline.StoreCenter.Database.Port,
		User: "issue36_trial_runtime", Password: "synthetic-trial-secret", Database: "store_center", MaxConnections: 4,
	}}
	require.Error(t, trial.validate(baseline), "plain HTTP must not admit a retained trial")
	baseline.Identity.IssuerURL, baseline.Identity.AuthorizationAPIURL = "https://localhost:18444", "https://localhost:18444"
	require.NoError(t, trial.validate(baseline))
	baseline.ProductAcquisitionDatabase = &DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "source_acquisition_runtime", Password: "synthetic", Database: "product_acquisition", MaxConnections: 2}
	require.Error(t, trial.validate(baseline), "provider acquisition must stay outside this trial")
}

func TestIssue36LocalTrialManifestRequiresExplicitIsolatedStoreBoundary(t *testing.T) {
	baseline := storeRuntimeConfig()
	baseline.StoreCenter.Database.Port = 5433
	baseline.Identity.IssuerURL = "https://localhost:18444"
	baseline.Identity.AuthorizationAPIURL = baseline.Identity.IssuerURL
	baseline.StoreCenter.Database.Database = "store_center"

	trial := map[string]any{
		"enabled":  true,
		"database": DatabaseConfig{Host: "127.0.0.1", Port: 5433, User: "issue36_trial_runtime", Password: "synthetic-trial-secret", Database: "store_center", MaxConnections: 4},
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
		ok   bool
	}{
		{name: "explicit isolated trial", ok: true},
		{name: "disabled marker", edit: func(m map[string]any) { m["enabled"] = false }},
		{name: "wrong role", edit: func(m map[string]any) {
			db := m["database"].(DatabaseConfig)
			db.User = "store_center_runtime"
			m["database"] = db
		}},
		{name: "wrong database", edit: func(m map[string]any) {
			db := m["database"].(DatabaseConfig)
			db.Database = "other"
			m["database"] = db
		}},
		{name: "remote host", edit: func(m map[string]any) {
			db := m["database"].(DatabaseConfig)
			db.Host = "198.51.100.7"
			m["database"] = db
		}},
		{name: "wide pool", edit: func(m map[string]any) {
			db := m["database"].(DatabaseConfig)
			db.MaxConnections = 9
			m["database"] = db
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyTrial := map[string]any{"enabled": trial["enabled"], "database": trial["database"]}
			if tc.edit != nil {
				tc.edit(copyTrial)
			}
			raw, err := json.Marshal(baseline)
			require.NoError(t, err)
			var manifest map[string]any
			require.NoError(t, json.Unmarshal(raw, &manifest))
			manifest["localTrial"] = copyTrial
			raw, err = json.Marshal(manifest)
			require.NoError(t, err)
			loaded, err := LoadConfig(writeManifest(t, string(raw)))
			if tc.ok {
				require.NoError(t, err)
				require.NotNil(t, loaded)
			} else {
				require.Error(t, err)
			}
		})
	}
}
