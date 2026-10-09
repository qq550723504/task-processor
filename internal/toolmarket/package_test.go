package toolmarket

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageRequiresTrustedFullAddressAndDigest(t *testing.T) {
	app := "https://capture.example.com:8443/capture/1688"
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	files := map[string]string{"manifest.json": `{"manifest_version":3,"name":"1688 商品采集","version":"0.1.0","externally_connectable":{"matches":["https://capture.example.com/capture/1688"]}}`, "background.js": "const app=" + `"` + app + `"` + ";", "popup.js": "x", "extractor.js": "x", "popup.html": "x", "popup.css": "x"}
	for n, v := range files {
		f, e := w.Create(n)
		require.NoError(t, e)
		_, e = f.Write([]byte(v))
		require.NoError(t, e)
	}
	require.NoError(t, w.Close())
	p := filepath.Join(t.TempDir(), "plugin.zip")
	require.NoError(t, os.WriteFile(p, b.Bytes(), 0600))
	sum := sha256.Sum256(b.Bytes())
	cfg := PackageConfig{Path: p, SHA256: hex.EncodeToString(sum[:]), CaptureAppURL: app}
	packageData, e := LoadPackage(cfg)
	require.NoError(t, e)
	require.Equal(t, b.Bytes(), packageData)
	cfg.CaptureAppURL = "https://capture.example.com:9443/capture/1688"
	_, e = LoadPackage(cfg)
	require.ErrorIs(t, e, ErrUnavailable)
	cfg.CaptureAppURL = app
	cfg.SHA256 = "bad"
	_, e = LoadPackage(cfg)
	require.ErrorIs(t, e, ErrUnavailable)
}
func TestDeploymentBuildPackage(t *testing.T) {
	directory := os.Getenv("TOOLMARKET_PACKAGE_TEST_DIR")
	if directory == "" {
		t.Skip("requires actual existing extension release build")
	}
	raw, e := os.ReadFile(filepath.Join(directory, "release.json"))
	require.NoError(t, e)
	var record struct {
		CaptureAppURL string `json:"captureAppUrl"`
		SHA256        string `json:"sha256"`
		Filename      string `json:"filename"`
	}
	require.NoError(t, json.Unmarshal(raw, &record))
	cfg := PackageConfig{Path: filepath.Join(directory, record.Filename), CaptureAppURL: record.CaptureAppURL, SHA256: record.SHA256}
	bytes, e := LoadPackage(cfg)
	require.NoError(t, e)
	require.NotEmpty(t, bytes)
	cfg.CaptureAppURL = "https://other.shuomi-test.com/capture/1688"
	_, e = LoadPackage(cfg)
	require.ErrorIs(t, e, ErrUnavailable)
}
