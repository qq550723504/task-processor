package currentapplication

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	tm "task-processor/internal/toolmarket"
)

type ToolMarketConfig struct {
	Database         DatabaseConfig `json:"database"`
	CaptureAppURL    string         `json:"captureAppURL"`
	PackageDirectory string         `json:"packageDirectory,omitempty"`
}

func (c *Config) validateToolMarket() error {
	if c.ToolMarket == nil {
		return nil
	}
	t := c.ToolMarket
	if err := t.Database.validate("toolMarket.database"); err != nil {
		return err
	}
	if t.Database.User != "tool_market_runtime" || t.Database.Database != "tool_market" || t.Database.MaxConnections > 4 {
		return errors.New("tool market requires its dedicated bounded runtime owner")
	}
	u, err := url.Parse(t.CaptureAppURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "/capture/1688" || u.RawQuery != "" || u.Fragment != "" || u.String() != t.CaptureAppURL {
		return errors.New("tool market requires the exact installation capture URL")
	}
	if c.Referrals.Enabled && t.CaptureAppURL != c.Referrals.PublicAppOrigin+"/capture/1688" {
		return errors.New("tool market capture URL differs from the installation public origin")
	}
	if t.PackageDirectory != "" && !filepath.IsAbs(t.PackageDirectory) {
		return errors.New("tool market package directory must be absolute")
	}
	return nil
}

// PackageConfig binds the existing trusted release record to the current
// installation receiver. Invalid/missing releases leave download unavailable.
func (t *ToolMarketConfig) PackageConfig() *tm.PackageConfig {
	if t == nil || t.PackageDirectory == "" {
		return nil
	}
	path := filepath.Join(t.PackageDirectory, "release.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(raw) > 4096 || validateJSONShape(raw) != nil {
		return nil
	}
	var record struct {
		SchemaVersion int    `json:"schemaVersion"`
		CaptureAppURL string `json:"captureAppUrl"`
		SHA256        string `json:"sha256"`
		Filename      string `json:"filename"`
	}
	if json.Unmarshal(raw, &record) != nil || record.SchemaVersion != 1 || record.CaptureAppURL != t.CaptureAppURL || record.Filename != "shuomi-1688-capture.zip" {
		return nil
	}
	return &tm.PackageConfig{Path: filepath.Join(t.PackageDirectory, record.Filename), SHA256: record.SHA256, CaptureAppURL: t.CaptureAppURL}
}
