package modules

type CategoryRestriction struct {
	CategoryID           int    `json:"category_id"`
	ForbiddenPrimarySpec int    `json:"forbidden_primary_spec"`
	DefaultPrimarySpec   int    `json:"default_primary_spec"`
	PlatformName         string `json:"platform_name"`
}
