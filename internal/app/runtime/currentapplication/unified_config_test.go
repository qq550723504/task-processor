package currentapplication

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUnifiedManifestRequiresNoSubscriptionReadOrQuotaPool(t *testing.T) {
	decode := func(raw []byte) error {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var cfg Config
		if err := decoder.Decode(&cfg); err != nil {
			return err
		}
		return cfg.validate()
	}

	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(validManifest()), &manifest))
	delete(manifest, "commercialDatabase")
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	err = decode(raw)
	require.NoError(t, err)
	manifest["commercialDatabase"] = map[string]any{}
	raw, err = json.Marshal(manifest)
	require.NoError(t, err)
	err = decode(raw)
	require.ErrorContains(t, err, "unknown field")
	delete(manifest, "commercialDatabase")
	manifest["storeCenter"] = map[string]any{"enabled": false, "quotaDatabase": map[string]any{}}
	raw, err = json.Marshal(manifest)
	require.NoError(t, err)
	err = decode(raw)
	require.ErrorContains(t, err, "unknown field")
}
