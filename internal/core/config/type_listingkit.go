package config

type ListingKitConfig struct {
	SheinSubmitDebugDumpDir string                  `mapstructure:"sheinSubmitDebugDumpDir" yaml:"sheinSubmitDebugDumpDir"`
	PlatformAdminUsers      []string                `mapstructure:"platformAdminUsers" yaml:"platformAdminUsers"`
	PlatformAdminRoles      []string                `mapstructure:"platformAdminRoles" yaml:"platformAdminRoles"`
	Zitadel                 ListingKitZitadelConfig `mapstructure:"zitadel" yaml:"zitadel"`
}

type ListingKitZitadelConfig struct {
	IssuerURL             string   `mapstructure:"issuerURL" yaml:"issuerURL"`
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
	AllowedUsernames                  []string `mapstructure:"allowedUsernames" yaml:"allowedUsernames"`
	LegacyUsernameAllowlistConfigured bool     `mapstructure:"-" yaml:"-"`
	AllowedRoles                      []string `mapstructure:"allowedRoles" yaml:"allowedRoles"`
}
