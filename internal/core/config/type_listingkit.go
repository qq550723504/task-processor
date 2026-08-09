package config

type ListingKitConfig struct {
	SheinSubmitDebugDumpDir string                  `mapstructure:"sheinSubmitDebugDumpDir" yaml:"sheinSubmitDebugDumpDir"`
	PlatformAdminUsers      []string                `mapstructure:"platformAdminUsers" yaml:"platformAdminUsers"`
	PlatformAdminRoles      []string                `mapstructure:"platformAdminRoles" yaml:"platformAdminRoles"`
	OwnerScopeRequired      bool                    `mapstructure:"ownerScopeRequired" yaml:"ownerScopeRequired"`
	Zitadel                 ListingKitZitadelConfig `mapstructure:"zitadel" yaml:"zitadel"`
}

type ListingKitZitadelConfig struct {
	IssuerURL             string                     `mapstructure:"issuerURL" yaml:"issuerURL"`
	ClientID              string                     `mapstructure:"clientID" yaml:"clientID"`
	ClientSecret          string                     `mapstructure:"clientSecret" yaml:"clientSecret"`
	TenantDirectoryToken  string                     `mapstructure:"tenantDirectoryToken" yaml:"tenantDirectoryToken"`
	MemberInvitationToken string                     `mapstructure:"memberInvitationToken" yaml:"memberInvitationToken"`
	ProjectID             string                     `mapstructure:"projectID" yaml:"projectID"`
	AuthorizationRequired bool                       `mapstructure:"authorizationRequired" yaml:"authorizationRequired"`
	AllowedTenantIDs      []string                   `mapstructure:"allowedTenantIDs" yaml:"allowedTenantIDs"`
	AllowedUserIDs        []string                   `mapstructure:"allowedUserIDs" yaml:"allowedUserIDs"`
	AllowedUsernames      []string                   `mapstructure:"allowedUsernames" yaml:"allowedUsernames"`
	AllowedRoles          []string                   `mapstructure:"allowedRoles" yaml:"allowedRoles"`
	SMS                   ListingKitZitadelSMSConfig `mapstructure:"sms" yaml:"sms"`
}

type ListingKitZitadelSMSConfig struct {
	SigningKey        string `mapstructure:"signingKey" yaml:"signingKey"`
	TencentSecretID   string `mapstructure:"tencentSecretID" yaml:"tencentSecretID"`
	TencentSecretKey  string `mapstructure:"tencentSecretKey" yaml:"tencentSecretKey"`
	TencentAppID      string `mapstructure:"tencentAppID" yaml:"tencentAppID"`
	TencentSignName   string `mapstructure:"tencentSignName" yaml:"tencentSignName"`
	TencentTemplateID string `mapstructure:"tencentTemplateID" yaml:"tencentTemplateID"`
}
