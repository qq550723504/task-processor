package currentapplication

import (
	"bytes"
	"context"
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
	"strconv"
	"strings"

	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
)

const (
	manifestSchemaVersion                = 1
	maximumManifestBytes                 = 64 * 1024
	maximumPlatformAdminAllowlistEntries = 64
	maximumPlatformAdminValueLength      = 256
)

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,62}$`)

type Config struct {
	SchemaVersion              int                           `json:"schemaVersion"`
	Listen                     ListenConfig                  `json:"listen"`
	Identity                   IdentityConfig                `json:"identity"`
	SourceAccountDatabase      DatabaseConfig                `json:"sourceAccountDatabase"`
	CommercialDatabase         DatabaseConfig                `json:"commercialDatabase"`
	CommercialOwnerDatabase    *DatabaseConfig               `json:"commercialOwnerDatabase,omitempty"`
	ProductAcquisitionDatabase *DatabaseConfig               `json:"productAcquisitionDatabase,omitempty"`
	ImageAgent                 *ImageAgentConfig             `json:"imageAgent,omitempty"`
	ProductAgent               *ProductAgentConfig           `json:"productAgent,omitempty"`
	Membership                 *MembershipConfig             `json:"membership,omitempty"`
	ListingKitAuthorization    ListingKitAuthorizationConfig `json:"listingKitAuthorization,omitempty"`
	Referrals                  ReferralsConfig               `json:"referrals"`
}

type ReferralsConfig struct {
	coreconfig.ReferralsConfig
	Database DatabaseConfig `json:"referralDatabase"`
}

// ImageAgentConfig enables only the current acquisition main-image path. Its
// database is the same owner database used by the Organization ImageAgent
// worker, opened with a bounded API runtime role, never the SRC role.
type ImageAgentConfig struct {
	Database                   DatabaseConfig `json:"database"`
	TemporalAddress            string         `json:"temporalAddress"`
	TemporalNamespace          string         `json:"temporalNamespace"`
	AllowedOrganizationIDs     []string       `json:"allowedOrganizationIds"`
	PublicBase                 string         `json:"publicBase"`
	Bucket                     string         `json:"bucket"`
	IsolatedTrialGeneratedURLs bool           `json:"isolatedTrialGeneratedURLs,omitempty"`
}

// ListingKitAuthorizationConfig carries only the platform-admin caller allowlists.
type ListingKitAuthorizationConfig struct {
	PlatformAdminUsers []string `json:"platformAdminUsers,omitempty"`
	PlatformAdminRoles []string `json:"platformAdminRoles,omitempty"`
}

type ListenConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type IdentityConfig struct {
	IssuerURL            string `json:"issuerURL"`
	AuthorizationAPIURL  string `json:"authorizationAPIURL"`
	ClientID             string `json:"clientID"`
	ClientSecret         string `json:"clientSecret"`
	ProjectID            string `json:"projectID"`
	TenantDirectoryToken string `json:"tenantDirectoryToken,omitempty"`
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
	if runtime.GOOS == "windows" && coreconfig.VerifyPrivateFiles(context.Background(), []string{path}) != nil {
		return nil, errors.New("current application manifest must be private")
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
	if cfg.Identity.TenantDirectoryToken != "" && !boundedValue(cfg.Identity.TenantDirectoryToken, 4096) {
		return errors.New("identity.tenantDirectoryToken must be trimmed and bounded")
	}
	if cfg.CommercialOwnerDatabase != nil && cfg.Referrals.Enabled && cfg.Identity.TenantDirectoryToken == "" {
		return errors.New("identity.tenantDirectoryToken is required for subscription purchase recovery")
	}
	if err := cfg.SourceAccountDatabase.validate("sourceAccountDatabase"); err != nil {
		return err
	}
	if err := cfg.CommercialDatabase.validate("commercialDatabase"); err != nil {
		return err
	}
	if cfg.SourceAccountDatabase.User != "source_account_runtime" || cfg.CommercialDatabase.User != "commercial_runtime" {
		return errors.New("current application database roles must be source_account_runtime and commercial_runtime")
	}
	if owner := cfg.CommercialOwnerDatabase; owner != nil {
		if err := owner.validate("commercialOwnerDatabase"); err != nil {
			return err
		}
		if owner.User != "commercial_owner_runtime" {
			return errors.New("commercial owner database requires commercial_owner_runtime")
		}
	}
	if cfg.Membership != nil {
		if err := cfg.Membership.validate(cfg.Identity); err != nil {
			return err
		}
		others := []DatabaseConfig{cfg.SourceAccountDatabase, cfg.CommercialDatabase}
		if cfg.CommercialOwnerDatabase != nil {
			others = append(others, *cfg.CommercialOwnerDatabase)
		}
		for _, other := range others {
			if cfg.Membership.Database.Host == other.Host && cfg.Membership.Database.Port == other.Port && cfg.Membership.Database.Database == other.Database {
				return errors.New("membership requires a dedicated database")
			}
		}
	}
	if product := cfg.ProductAcquisitionDatabase; product != nil {
		if err := product.validate("productAcquisitionDatabase"); err != nil {
			return err
		}
		if product.User != "source_acquisition_runtime" || product.MaxConnections > 8 {
			return errors.New("product acquisition requires source_acquisition_runtime and at most 8 connections")
		}
		others := []DatabaseConfig{cfg.SourceAccountDatabase, cfg.CommercialDatabase}
		if cfg.CommercialOwnerDatabase != nil {
			others = append(others, *cfg.CommercialOwnerDatabase)
		}
		for _, other := range others {
			if product.Host == other.Host && product.Port == other.Port && product.Database == other.Database {
				return errors.New("product acquisition requires a dedicated Product database")
			}
		}
	}
	if image := cfg.ImageAgent; image != nil {
		if cfg.ProductAcquisitionDatabase == nil {
			return errors.New("image agent requires current product acquisition")
		}
		if err := image.Database.validate("imageAgent.database"); err != nil {
			return err
		}
		if image.Database.User != "image_agent_runtime" || image.Database.MaxConnections > 8 {
			return errors.New("image agent requires image_agent_runtime and at most 8 connections")
		}
		for _, other := range []*DatabaseConfig{&cfg.SourceAccountDatabase, &cfg.CommercialDatabase, cfg.CommercialOwnerDatabase, cfg.ProductAcquisitionDatabase, &cfg.Referrals.Database} {
			if other != nil && image.Database.Host == other.Host && image.Database.Port == other.Port && image.Database.Database == other.Database {
				return errors.New("image agent requires a dedicated owner database")
			}
		}
		if cfg.Membership != nil && image.Database.Host == cfg.Membership.Database.Host && image.Database.Port == cfg.Membership.Database.Port && image.Database.Database == cfg.Membership.Database.Database {
			return errors.New("image agent requires a dedicated owner database")
		}
		host, port, err := net.SplitHostPort(image.TemporalAddress)
		if err != nil || host != "127.0.0.1" {
			return errors.New("image agent Temporal address must use explicit IPv4 loopback")
		}
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 || !boundedValue(image.TemporalNamespace, 128) {
			return errors.New("image agent Temporal endpoint and namespace must be bounded")
		}
		if len(image.AllowedOrganizationIDs) == 0 || len(image.AllowedOrganizationIDs) > 64 || !boundedValue(image.Bucket, 128) {
			return errors.New("image agent organization allowlist and bucket are required")
		}
		seen := make(map[string]struct{}, len(image.AllowedOrganizationIDs))
		for _, id := range image.AllowedOrganizationIDs {
			if !boundedValue(id, 256) {
				return errors.New("image agent organization allowlist is invalid")
			}
			if _, duplicate := seen[id]; duplicate {
				return errors.New("image agent organization allowlist contains a duplicate")
			}
			seen[id] = struct{}{}
		}
		if image.IsolatedTrialGeneratedURLs {
			if _, err := imageagent.NewIsolatedTrialGeneratedURLPolicy(image.PublicBase, image.Bucket); err != nil {
				return errors.New("image agent isolated trial generated URL base is invalid")
			}
		} else if _, err := imageagent.ValidateSafeImageURL(image.PublicBase); err != nil {
			return errors.New("image agent public base must be a safe public URL")
		}
	}
	if cfg.ProductAgent != nil {
		if err := cfg.ProductAgent.validate(cfg); err != nil {
			return err
		}
	}
	if err := validatePlatformAdminAllowlist("listingKitAuthorization.platformAdminUsers", cfg.ListingKitAuthorization.PlatformAdminUsers); err != nil {
		return err
	}
	if err := validatePlatformAdminAllowlist("listingKitAuthorization.platformAdminRoles", cfg.ListingKitAuthorization.PlatformAdminRoles); err != nil {
		return err
	}
	if cfg.Referrals.Enabled {
		if err := cfg.Referrals.Database.validate("referrals.referralDatabase"); err != nil {
			return err
		}
		if cfg.Referrals.Database.User != "referral_runtime" || cfg.Referrals.Issuer != cfg.Identity.IssuerURL {
			return errors.New("referrals requires its runtime role and the current identity issuer")
		}
	}
	if cfg.Membership != nil {
		for _, other := range []*DatabaseConfig{cfg.ProductAcquisitionDatabase} {
			if other != nil && cfg.Membership.Database.Host == other.Host && cfg.Membership.Database.Port == other.Port && cfg.Membership.Database.Database == other.Database {
				return errors.New("membership requires a dedicated database")
			}
		}
		if cfg.Referrals.Enabled && cfg.Membership.Database.Host == cfg.Referrals.Database.Host && cfg.Membership.Database.Port == cfg.Referrals.Database.Port && cfg.Membership.Database.Database == cfg.Referrals.Database.Database {
			return errors.New("membership requires a dedicated database")
		}
	}
	return nil
}

func validatePlatformAdminAllowlist(name string, values []string) error {
	if len(values) > maximumPlatformAdminAllowlistEntries {
		return fmt.Errorf("%s exceeds the entry limit", name)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !boundedValue(value, maximumPlatformAdminValueLength) {
			return fmt.Errorf("%s entries must be non-empty, trimmed, and bounded", name)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains a duplicate entry", name)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateLoopbackURL(name, raw string) (*url.URL, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasSuffix(raw, "?") || strings.HasSuffix(raw, "#") {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP(S) URL", name)
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP(S) URL", name)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, fmt.Errorf("identity.%s must be a bounded absolute loopback HTTP(S) URL", name)
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
	core := &coreconfig.Config{
		Referrals: cfg.Referrals.ReferralsConfig,
		Workbench: coreconfig.WorkbenchConfig{Enabled: true},
		ListingKit: coreconfig.ListingKitConfig{
			PlatformAdminUsers: append([]string(nil), cfg.ListingKitAuthorization.PlatformAdminUsers...),
			PlatformAdminRoles: append([]string(nil), cfg.ListingKitAuthorization.PlatformAdminRoles...),
			Zitadel: coreconfig.ListingKitZitadelConfig{
				IssuerURL: cfg.Identity.IssuerURL, AuthorizationAPIURL: cfg.Identity.AuthorizationAPIURL,
				ClientID: cfg.Identity.ClientID, ClientSecret: cfg.Identity.ClientSecret, ProjectID: cfg.Identity.ProjectID,
				TenantDirectoryToken:  cfg.Identity.TenantDirectoryToken,
				AuthorizationRequired: true,
			},
		},
	}
	if image := cfg.ImageAgent; image != nil {
		core.ImageAgent.Admission = coreconfig.ImageAgentAdmissionConfig{Enabled: true, AllowedTenantIDs: append([]string(nil), image.AllowedOrganizationIDs...)}
		core.ImageAgent.ArtifactStore = coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: image.PublicBase, IsolatedTrialGeneratedURLs: image.IsolatedTrialGeneratedURLs, S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: image.Bucket}}
	}
	return core
}
