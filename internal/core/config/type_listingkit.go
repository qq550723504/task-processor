package config

type ListingKitConfig struct {
	SheinSubmitDebugDumpDir        string                      `mapstructure:"sheinSubmitDebugDumpDir" yaml:"sheinSubmitDebugDumpDir"`
	GenerationUsageLedgerEnabled   bool                        `mapstructure:"generationUsageLedgerEnabled" yaml:"generationUsageLedgerEnabled"`
	GenerationUsageLedgerTenantIDs []string                    `mapstructure:"generationUsageLedgerTenantIDs" yaml:"generationUsageLedgerTenantIDs"`
	PlatformAdminUsers             []string                    `mapstructure:"platformAdminUsers" yaml:"platformAdminUsers"`
	PlatformAdminRoles             []string                    `mapstructure:"platformAdminRoles" yaml:"platformAdminRoles"`
	ImageUpload                    ListingKitImageUploadConfig `mapstructure:"imageUpload" yaml:"imageUpload"`
	Zitadel                        ListingKitZitadelConfig     `mapstructure:"zitadel" yaml:"zitadel"`
}

type ListingKitImageUploadConfig struct {
	Provider string                           `mapstructure:"provider" yaml:"provider"`
	Local    ListingKitImageUploadLocalConfig `mapstructure:"local" yaml:"local"`
	S3       ListingKitImageUploadS3Config    `mapstructure:"s3" yaml:"s3"`
}

type ListingKitImageUploadLocalConfig struct {
	RootDir string `mapstructure:"rootDir" yaml:"rootDir"`
}

type ListingKitImageUploadS3Config struct {
	Bucket          string `mapstructure:"bucket" yaml:"bucket"`
	Region          string `mapstructure:"region" yaml:"region"`
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `mapstructure:"accessKeyID" yaml:"accessKeyID"`
	SecretAccessKey string `mapstructure:"secretAccessKey" yaml:"secretAccessKey"`
	UsePathStyle    bool   `mapstructure:"usePathStyle" yaml:"usePathStyle"`
	PublicBase      string `mapstructure:"publicBase" yaml:"publicBase"`
}

type ListingKitZitadelConfig struct {
	IssuerURL             string   `mapstructure:"issuerURL" yaml:"issuerURL"`
	AuthorizationAPIURL   string   `mapstructure:"authorizationAPIURL" yaml:"authorizationAPIURL"`
	ClientID              string   `mapstructure:"clientID" yaml:"clientID"`
	ClientSecret          string   `mapstructure:"clientSecret" yaml:"clientSecret"`
	TenantDirectoryToken  string   `mapstructure:"tenantDirectoryToken" yaml:"tenantDirectoryToken"`
	MemberInvitationToken string   `mapstructure:"memberInvitationToken" yaml:"memberInvitationToken"`
	ProjectID             string   `mapstructure:"projectID" yaml:"projectID"`
	AuthorizationRequired bool     `mapstructure:"authorizationRequired" yaml:"authorizationRequired"`
	AllowedTenantIDs      []string `mapstructure:"allowedTenantIDs" yaml:"allowedTenantIDs"`
	AllowedUserIDs        []string `mapstructure:"allowedUserIDs" yaml:"allowedUserIDs"`
	// AllowedUsernames is retained only to detect and reject obsolete configuration.
	// It must never participate in authorization decisions.
	AllowedUsernames                  []string                   `mapstructure:"allowedUsernames" yaml:"allowedUsernames"`
	LegacyUsernameAllowlistConfigured bool                       `mapstructure:"-" yaml:"-"`
	AllowedRoles                      []string                   `mapstructure:"allowedRoles" yaml:"allowedRoles"`
	SMS                               ListingKitZitadelSMSConfig `mapstructure:"sms" yaml:"sms"`
}

type ListingKitZitadelSMSConfig struct {
	SigningKey        string `mapstructure:"signingKey" yaml:"signingKey"`
	TencentSecretID   string `mapstructure:"tencentSecretID" yaml:"tencentSecretID"`
	TencentSecretKey  string `mapstructure:"tencentSecretKey" yaml:"tencentSecretKey"`
	TencentAppID      string `mapstructure:"tencentAppID" yaml:"tencentAppID"`
	TencentSignName   string `mapstructure:"tencentSignName" yaml:"tencentSignName"`
	TencentTemplateID string `mapstructure:"tencentTemplateID" yaml:"tencentTemplateID"`
}
