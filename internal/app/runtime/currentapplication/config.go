package currentapplication

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	coreconfig "task-processor/internal/core/config"
)

const (
	manifestSchemaVersion = 1
	maximumManifestBytes  = 64 * 1024
)

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,62}$`)

type Config struct {
	SchemaVersion              int             `json:"schemaVersion"`
	Listen                     ListenConfig    `json:"listen"`
	Identity                   IdentityConfig  `json:"identity"`
	SourceAccountDatabase      DatabaseConfig  `json:"sourceAccountDatabase"`
	CommercialDatabase         DatabaseConfig  `json:"commercialDatabase"`
	ProductAcquisitionDatabase *DatabaseConfig `json:"productAcquisitionDatabase,omitempty"`
}

type ListenConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type IdentityConfig struct {
	IssuerURL           string `json:"issuerURL"`
	AuthorizationAPIURL string `json:"authorizationAPIURL"`
	ClientID            string `json:"clientID"`
	ClientSecret        string `json:"clientSecret"`
	ProjectID           string `json:"projectID"`
}

type DatabaseConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"password"`
	Database       string `json:"database"`
	MaxConnections int    `json:"maxConnections"`
}

func LoadConfig(path string) (*Config, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("current application manifest path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect current application manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("current application manifest must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("current application manifest must not be accessible by group or others")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open current application manifest: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read current application manifest: %w", err)
	}
	if len(data) > maximumManifestBytes {
		return nil, errors.New("current application manifest exceeds size limit")
	}
	if err := validateJSONShape(data); err != nil {
		return nil, fmt.Errorf("parse current application manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode current application manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode current application manifest: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateJSONShape(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := readUniqueJSONValue(decoder); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func readUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := readUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("object is not terminated")
		}
	case '[':
		for decoder.More() {
			if err := readUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("array is not terminated")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("manifest contains trailing JSON value")
		}
		return err
	}
	return nil
}

func (cfg *Config) validate() error {
	if cfg == nil || cfg.SchemaVersion != manifestSchemaVersion {
		return fmt.Errorf("unsupported current application manifest schema version")
	}
	if cfg.Listen.Host != "127.0.0.1" || cfg.Listen.Port < 1 || cfg.Listen.Port > 65535 {
		return errors.New("current application listener must use 127.0.0.1 and a valid explicit port")
	}
	issuer, err := validateLoopbackURL("issuerURL", cfg.Identity.IssuerURL)
	if err != nil {
		return err
	}
	authorization, err := validateLoopbackURL("authorizationAPIURL", cfg.Identity.AuthorizationAPIURL)
	if err != nil {
		return err
	}
	if !strings.EqualFold(issuer.Scheme, authorization.Scheme) || !strings.EqualFold(issuer.Host, authorization.Host) {
		return errors.New("identity issuer and authorization API must have the same loopback origin")
	}
	for name, value := range map[string]string{
		"identity.clientID": cfg.Identity.ClientID, "identity.clientSecret": cfg.Identity.ClientSecret, "identity.projectID": cfg.Identity.ProjectID,
	} {
		if !boundedValue(value, 512) {
			return fmt.Errorf("%s is required and must be bounded", name)
		}
	}
	if err := cfg.SourceAccountDatabase.validate("sourceAccountDatabase"); err != nil {
		return err
	}
	if err := cfg.CommercialDatabase.validate("commercialDatabase"); err != nil {
		return err
	}
	if cfg.SourceAccountDatabase.User != "source_account_runtime" || cfg.CommercialDatabase.User != "commercial_reader" {
		return errors.New("current application database roles must be source_account_runtime and commercial_reader")
	}
	if product := cfg.ProductAcquisitionDatabase; product != nil {
		if err := product.validate("productAcquisitionDatabase"); err != nil {
			return err
		}
		if product.User != "source_acquisition_runtime" || product.MaxConnections > 8 {
			return errors.New("product acquisition requires source_acquisition_runtime and at most 8 connections")
		}
		for _, other := range []DatabaseConfig{cfg.SourceAccountDatabase, cfg.CommercialDatabase} {
			if product.Host == other.Host && product.Port == other.Port && product.Database == other.Database {
				return errors.New("product acquisition requires a dedicated Product database")
			}
		}
	}
	return nil
}

func validateLoopbackURL(name, raw string) (*url.URL, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasSuffix(raw, "?") || strings.HasSuffix(raw, "#") {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP URL", name)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP URL", name)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP URL", name)
	}
	if parsed.Port() == "" || len(raw) > 2048 {
		return nil, fmt.Errorf("identity.%s must include an explicit loopback port", name)
	}
	return parsed, nil
}

func (cfg DatabaseConfig) validate(name string) error {
	if cfg.Host != "127.0.0.1" || cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("%s must use an explicit IPv4 loopback endpoint", name)
	}
	if !boundedValue(cfg.User, 128) || !boundedDatabasePassword(cfg.Password) || !databaseNamePattern.MatchString(cfg.Database) {
		return fmt.Errorf("%s credentials and database name are required and must be bounded", name)
	}
	if cfg.MaxConnections < 1 || cfg.MaxConnections > 20 {
		return fmt.Errorf("%s.maxConnections must be between 1 and 20", name)
	}
	return nil
}

func boundedDatabasePassword(value string) bool {
	// pgx keyword/value DSNs treat VT and FF as separators too.
	return boundedValue(value, 1024) && !strings.ContainsAny(value, " ='\\\t\v\f")
}

func boundedValue(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
}

func (cfg *Config) ListenAddress() string {
	if cfg == nil {
		return ""
	}
	return net.JoinHostPort(cfg.Listen.Host, fmt.Sprint(cfg.Listen.Port))
}

func (cfg *Config) CoreConfig() *coreconfig.Config {
	if cfg == nil {
		return nil
	}
	return &coreconfig.Config{
		Workbench: coreconfig.WorkbenchConfig{Enabled: true},
		ListingKit: coreconfig.ListingKitConfig{Zitadel: coreconfig.ListingKitZitadelConfig{
			IssuerURL: cfg.Identity.IssuerURL, AuthorizationAPIURL: cfg.Identity.AuthorizationAPIURL,
			ClientID: cfg.Identity.ClientID, ClientSecret: cfg.Identity.ClientSecret, ProjectID: cfg.Identity.ProjectID,
			AuthorizationRequired: true,
		}},
	}
}
