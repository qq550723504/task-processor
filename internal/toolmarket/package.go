package toolmarket

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"strings"
)

type PackageConfig struct{ Path, SHA256, CaptureAppURL string }

const MaxPackageBytes = 2 << 20

// LoadPackage binds immutable bytes to a trusted deployment build record.
// The caller supplies the full receiver URL and digest from that same build.
func LoadPackage(c PackageConfig) ([]byte, error) {
	u, e := url.Parse(c.CaptureAppURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "/capture/1688" || u.RawQuery != "" || u.Fragment != "" || u.String() != c.CaptureAppURL {
		return nil, ErrUnavailable
	}
	f, e := os.Open(c.Path)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, MaxPackageBytes+1))
	if e != nil || len(raw) > MaxPackageBytes {
		return nil, ErrUnavailable
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != c.SHA256 {
		return nil, ErrUnavailable
	}
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil || len(z.File) != 6 {
		return nil, ErrUnavailable
	}
	names := map[string]bool{"manifest.json": true, "background.js": true, "popup.js": true, "extractor.js": true, "popup.html": true, "popup.css": true}
	files := map[string][]byte{}
	total := 0
	for _, entry := range z.File {
		if !names[entry.Name] || entry.UncompressedSize64 > MaxPackageBytes || entry.Mode()&os.ModeSymlink != 0 {
			return nil, ErrUnavailable
		}
		delete(names, entry.Name)
		reader, e := entry.Open()
		if e != nil {
			return nil, ErrUnavailable
		}
		b, e := io.ReadAll(io.LimitReader(reader, MaxPackageBytes+1))
		_ = reader.Close()
		total += len(b)
		if e != nil || len(b) > MaxPackageBytes || total > 4*MaxPackageBytes {
			return nil, ErrUnavailable
		}
		files[entry.Name] = b
	}
	var manifest struct {
		Version  int    `json:"manifest_version"`
		Name     string `json:"name"`
		External struct {
			Matches []string `json:"matches"`
		} `json:"externally_connectable"`
	}
	if json.Unmarshal(files["manifest.json"], &manifest) != nil || manifest.Version != 3 || manifest.Name != "1688 商品采集" || len(manifest.External.Matches) != 1 || manifest.External.Matches[0] != "https://"+u.Hostname()+"/capture/1688" {
		return nil, ErrUnavailable
	}
	quoted, _ := json.Marshal(c.CaptureAppURL)
	if !strings.Contains(string(files["background.js"]), string(quoted)) {
		return nil, ErrUnavailable
	}
	return raw, nil
}
