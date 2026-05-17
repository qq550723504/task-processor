package config

type ListingKitConfig struct {
	StudioAsyncJobStorePath string                  `mapstructure:"studioAsyncJobStorePath" yaml:"studioAsyncJobStorePath"`
	SheinSubmitDebugDumpDir string                  `mapstructure:"sheinSubmitDebugDumpDir" yaml:"sheinSubmitDebugDumpDir"`
	PlatformAdminUsers      []string                `mapstructure:"platformAdminUsers" yaml:"platformAdminUsers"`
	PlatformAdminRoles      []string                `mapstructure:"platformAdminRoles" yaml:"platformAdminRoles"`
	OwnerScopeRequired      bool                    `mapstructure:"ownerScopeRequired" yaml:"ownerScopeRequired"`
	Zitadel                 ListingKitZitadelConfig `mapstructure:"zitadel" yaml:"zitadel"`
}

type ListingKitZitadelConfig struct {
	IssuerURL             string   `mapstructure:"issuerURL" yaml:"issuerURL"`
	ClientID              string   `mapstructure:"clientID" yaml:"clientID"`
	ClientSecret          string   `mapstructure:"clientSecret" yaml:"clientSecret"`
	AuthRequired          bool     `mapstructure:"authRequired" yaml:"authRequired"`
	AuthorizationRequired bool     `mapstructure:"authorizationRequired" yaml:"authorizationRequired"`
	AllowedTenantIDs      []string `mapstructure:"allowedTenantIDs" yaml:"allowedTenantIDs"`
	AllowedUserIDs        []string `mapstructure:"allowedUserIDs" yaml:"allowedUserIDs"`
	AllowedUsernames      []string `mapstructure:"allowedUsernames" yaml:"allowedUsernames"`
	AllowedRoles          []string `mapstructure:"allowedRoles" yaml:"allowedRoles"`
}
