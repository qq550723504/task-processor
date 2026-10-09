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
	"time"

	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	topupconfig "task-processor/internal/integration/wallettopup/config"
)

const (
	manifestSchemaVersion                = 1
	maximumManifestBytes                 = 64 * 1024
	maximumPlatformAdminAllowlistEntries = 64
	maximumPlatformAdminValueLength      = 256
)

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,62}$`)

type Config struct {
	NotificationCenterDatabase *DatabaseConfig                           `json:"notificationCenterDatabase,omitempty"`
	Knowledge                  *KnowledgeConfig                          `json:"knowledge,omitempty"`
	StoreCenter                *StoreCenterConfig                        `json:"storeCenter,omitempty"`
	LocalTrial                 *LocalTrialConfig                         `json:"localTrial,omitempty"`
	SchemaVersion              int                                       `json:"schemaVersion"`
	Listen                     ListenConfig                              `json:"listen"`
	Identity                   IdentityConfig                            `json:"identity"`
	SourceAccountDatabase      DatabaseConfig                            `json:"sourceAccountDatabase"`
	CommercialOwnerDatabase    *DatabaseConfig                           `json:"commercialOwnerDatabase,omitempty"`
	MoneyOwnerDatabase         *DatabaseConfig                           `json:"moneyOwnerDatabase,omitempty"`
	WalletTopUp                topupconfig.Config                        `json:"walletTopUp,omitempty"`
	ProductAcquisitionDatabase *DatabaseConfig                           `json:"productAcquisitionDatabase,omitempty"`
	ProductCollections         bool                                      `json:"productCollections,omitempty"`
	SupplyChain                *SupplyChainConfig                        `json:"supplyChain,omitempty"`
	SourceMedia                *coreconfig.ImageAgentArtifactStoreConfig `json:"sourceMedia,omitempty"`
	ImageAgent                 *ImageAgentConfig                         `json:"imageAgent,omitempty"`
	ProductAgent               *ProductAgentConfig                       `json:"productAgent,omitempty"`
	AIWorkbench                *AIWorkbenchConfig                        `json:"aiWorkbench,omitempty"`
	AccountAuditUsage          *AccountAuditUsageConfig                  `json:"accountAuditUsage,omitempty"`
	Membership                 *MembershipConfig                         `json:"membership,omitempty"`
	ListingKitAuthorization    ListingKitAuthorizationConfig             `json:"listingKitAuthorization,omitempty"`
	Referrals                  ReferralsConfig                           `json:"referrals"`
	// BrowserCollector enables the standalone anonymous public 1688 browser
	// collector (design D13). Omitted means the application keeps the existing
	// anonymous public HTTP provider unchanged.
	BrowserCollector *BrowserCollectorConfig `json:"browserCollector,omitempty"`
}

// BrowserCollectorConfig is the opt-in wiring to the browser collector
// process. Credential is a service-to-service credential between this
// application and the collector; it is never a user or tenant credential.
type BrowserCollectorConfig struct {
	Endpoint   string        `json:"endpoint"`
	Credential string        `json:"credential"`
	Timeout    time.Duration `json:"timeout,omitempty"`
}

type ReferralsConfig struct {
	coreconfig.ReferralsConfig
	Database DatabaseConfig `json:"referralDatabase"`
}

// ImageAgentConfig enables only the current acquisition main-image path. Its
// database is the same owner database used by the Organization ImageAgent
// worker, opened with a bounded API runtime role, never the SRC role.
type ImageAgentConfig struct {
	Generation                 coreconfig.ImageAgentGenerationConfig `json:"generation,omitempty"`
	Database                   DatabaseConfig                        `json:"database"`
	TemporalAddress            string                                `json:"temporalAddress"`
	TemporalNamespace          string                                `json:"temporalNamespace"`
	AllowedOrganizationIDs     []string                              `json:"allowedOrganizationIds"`
	PublicBase                 string                                `json:"publicBase"`
	Bucket                     string                                `json:"bucket"`
	IsolatedTrialGeneratedURLs bool                                  `json:"isolatedTrialGeneratedURLs,omitempty"`
}

// AccountAuditUsageConfig supplies bounded, read-only pools for the existing
// invocation owners not already supplied by their enabled Agent runtime.
type AccountAuditUsageConfig struct {
	Image   DatabaseConfig `json:"image"`
	Product DatabaseConfig `json:"product"`
}

func (a *AccountAuditUsageConfig) validate(cfg *Config) error {
	if a == nil {
		return nil
	}
	for _, target := range []struct {
		name, database, user string
		value                DatabaseConfig
		execution            *DatabaseConfig
	}{
		{"image", "image_agent", "account_audit_image_reader", a.Image, nil},
		{"product", "product_agent", "account_audit_product_reader", a.Product, nil},
	} {
		if target.name == "image" && cfg.ImageAgent != nil {
			target.execution = &cfg.ImageAgent.Database
		}
		if target.name == "product" && cfg.ProductAgent != nil && cfg.ProductAgent.Enabled {
			target.execution = &cfg.ProductAgent.Database
		}
		if target.execution != nil {
			if target.value != (DatabaseConfig{}) {
				return fmt.Errorf("account audit %s cannot duplicate its Agent execution source", target.name)
			}
			if target.execution.Database != target.database || target.execution.Host != cfg.SourceAccountDatabase.Host || target.execution.Port != cfg.SourceAccountDatabase.Port {
				return fmt.Errorf("account audit %s execution source must use its current local owner", target.name)
			}
			continue
		}
		if err := target.value.validate("accountAuditUsage." + target.name); err != nil {
			return err
		}
		if target.value.Database != target.database || target.value.User != target.user || target.value.MaxConnections > 2 || target.value.Host != cfg.SourceAccountDatabase.Host || target.value.Port != cfg.SourceAccountDatabase.Port {
			return fmt.Errorf("account audit %s requires its dedicated local read-only owner", target.name)
		}
	}
	if a.Image == (DatabaseConfig{}) && a.Product == (DatabaseConfig{}) {
		return errors.New("account audit read-only sources must declare at least one owner")
	}
	return nil
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
	if cfg.NotificationCenterDatabase != nil {
		if err := cfg.NotificationCenterDatabase.validate("notificationCenterDatabase"); err != nil {
			return err
		}
		if cfg.NotificationCenterDatabase.User != "notification_center_runtime" || cfg.NotificationCenterDatabase.MaxConnections > 4 {
			return errors.New("notification center requires its restricted role and at most four connections")
		}
		other := []*DatabaseConfig{&cfg.SourceAccountDatabase, cfg.CommercialOwnerDatabase, cfg.MoneyOwnerDatabase, cfg.ProductAcquisitionDatabase}
		if cfg.StoreCenter != nil {
			other = append(other, &cfg.StoreCenter.Database)
		}
		if cfg.Knowledge != nil {
			other = append(other, &cfg.Knowledge.Database)
		}
		if cfg.Membership != nil {
			other = append(other, &cfg.Membership.Database)
		}
		if cfg.Referrals.Enabled {
			other = append(other, &cfg.Referrals.Database)
		}
		if cfg.AIWorkbench != nil {
			other = append(other, &cfg.AIWorkbench.Database)
		}
		if cfg.ProductAgent != nil {
			other = append(other, &cfg.ProductAgent.Database, &cfg.ProductAgent.ReviewDatabase, &cfg.ProductAgent.AssetDatabase)
		}
		if cfg.ImageAgent != nil {
			other = append(other, &cfg.ImageAgent.Database)
		}
		if cfg.SupplyChain != nil {
			other = append(other, &cfg.SupplyChain.AssetDatabase)
		}
		for _, db := range other {
			n := cfg.NotificationCenterDatabase
			if db != nil && db.Host == n.Host && db.Port == n.Port && db.Database == n.Database {
				return errors.New("notification center requires a dedicated database")
			}
		}
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
		return errors.New("identity.tenantDirectoryToken is required for financial recovery")
	}
	if err := cfg.SourceAccountDatabase.validate("sourceAccountDatabase"); err != nil {
		return err
	}
	if cfg.SourceAccountDatabase.User != "source_account_runtime" {
		return errors.New("current application database roles must be source_account_runtime")
	}
	if owner := cfg.CommercialOwnerDatabase; owner != nil {
		if err := owner.validate("commercialOwnerDatabase"); err != nil {
			return err
		}
		if owner.User != "commercial_owner_runtime" {
			return errors.New("commercial owner database requires commercial_owner_runtime")
		}
	}
	if cfg.WalletTopUp.Alipay.Enabled || cfg.WalletTopUp.WeChat.Enabled {
		if cfg.MoneyOwnerDatabase == nil || cfg.CommercialOwnerDatabase == nil || cfg.Identity.TenantDirectoryToken == "" {
			return errors.New("wallet top-up requires money and commercial owner databases and current directory authorization")
		}
	}
	if owner := cfg.MoneyOwnerDatabase; owner != nil {
		if err := owner.validate("moneyOwnerDatabase"); err != nil {
			return err
		}
		if owner.User != "money_owner_runtime" || cfg.CommercialOwnerDatabase == nil {
			return errors.New("money owner database requires money_owner_runtime and commercial owner")
		}
		if cfg.Referrals.Enabled && (owner.Host != cfg.Referrals.Database.Host || owner.Port != cfg.Referrals.Database.Port || owner.Database != cfg.Referrals.Database.Database) {
			return errors.New("money owner must use the same canonical settlement database read by referrals")
		}
	}
	if cfg.Membership != nil {
		if err := cfg.Membership.validate(cfg.Identity); err != nil {
			return err
		}
		others := []DatabaseConfig{cfg.SourceAccountDatabase}
		if cfg.CommercialOwnerDatabase != nil {
			others = append(others, *cfg.CommercialOwnerDatabase)
		}
		for _, other := range others {
			if cfg.Membership.Database.Host == other.Host && cfg.Membership.Database.Port == other.Port && cfg.Membership.Database.Database == other.Database {
				return errors.New("membership requires a dedicated database")
			}
		}
	}
	if cfg.ProductCollections && cfg.ProductAcquisitionDatabase == nil {
		return errors.New("product collections require the current Product database")
	}
	if err := cfg.validateSupplyChain(); err != nil {
		return err
	}
	if cfg.SourceMedia != nil {
		if !cfg.ProductCollections || cfg.SupplyChain == nil {
			return errors.New("source media requires current collections and supply chain")
		}
		if err := ValidateSourceMediaStorage(*cfg.SourceMedia); err != nil {
			return errors.New("source media storage configuration unavailable")
		}
	}
	if product := cfg.ProductAcquisitionDatabase; product != nil {
		if cfg.CommercialOwnerDatabase == nil {
			return errors.New("product acquisition requires the canonical resource owner database")
		}
		if err := product.validate("productAcquisitionDatabase"); err != nil {
			return err
		}
		if product.User != "source_acquisition_runtime" || product.MaxConnections > 8 {
			return errors.New("product acquisition requires source_acquisition_runtime and at most 8 connections")
		}
		others := []DatabaseConfig{cfg.SourceAccountDatabase}
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
		if image.Generation != (coreconfig.ImageAgentGenerationConfig{}) && (!image.Generation.Configured() || cfg.CommercialOwnerDatabase == nil) {
			return errors.New("image generation requires an explicit versioned points price and commercial owner database")
		}
		if cfg.ProductAcquisitionDatabase == nil {
			return errors.New("image agent requires current product acquisition")
		}
		if err := image.Database.validate("imageAgent.database"); err != nil {
			return err
		}
		if image.Database.User != "image_agent_runtime" || image.Database.MaxConnections > 8 {
			return errors.New("image agent requires image_agent_runtime and at most 8 connections")
		}
		for _, other := range []*DatabaseConfig{&cfg.SourceAccountDatabase, cfg.CommercialOwnerDatabase, cfg.ProductAcquisitionDatabase, &cfg.Referrals.Database} {
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
	if err := cfg.validateStoreCenter(); err != nil {
		return err
	}
	if err := cfg.LocalTrial.validate(cfg); err != nil {
		return err
	}
	if err := cfg.validateKnowledge(); err != nil {
		return err
	}
	if cfg.ProductAgent != nil {
		if err := cfg.ProductAgent.validate(cfg); err != nil {
			return err
		}
	}
	if cfg.AIWorkbench != nil {
		if err := cfg.AIWorkbench.validate(cfg); err != nil {
			return err
		}
	}
	if err := cfg.AccountAuditUsage.validate(cfg); err != nil {
		return err
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
		WalletTopUp: cfg.WalletTopUp,
		Referrals:   cfg.Referrals.ReferralsConfig,
		Workbench:   coreconfig.WorkbenchConfig{Enabled: true},
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
	if collector := cfg.BrowserCollector; collector != nil {
		core.BrowserCollector = coreconfig.BrowserCollectorConfig{
			Endpoint:   collector.Endpoint,
			Credential: collector.Credential,
			Timeout:    collector.Timeout,
		}
	}
	if image := cfg.ImageAgent; image != nil {
		core.ImageAgent.Generation = image.Generation
		core.ImageAgent.Admission = coreconfig.ImageAgentAdmissionConfig{Enabled: true, AllowedTenantIDs: append([]string(nil), image.AllowedOrganizationIDs...)}
		core.ImageAgent.ArtifactStore = coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: image.PublicBase, IsolatedTrialGeneratedURLs: image.IsolatedTrialGeneratedURLs, S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: image.Bucket}}
	}
	if cfg.SourceMedia != nil {
		core.ProductCollectionSourceMedia = *cfg.SourceMedia
	}
	return core
}
