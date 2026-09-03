package listingkit

type SourceReference struct {
	Key      string `json:"key,omitempty"`
	Type     string `json:"type,omitempty"`
	Platform string `json:"platform,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

type GenerateRequest struct {
	TenantID   string `json:"tenant_id,omitempty"`
	ProductKey string `json:"product_key"`
	// SourceSnapshotVersion is populated only by trusted source handoff code
	// after Catalog publication. It is copied onto Task and never accepted from
	// or exposed to the caller-facing request JSON.
	SourceSnapshotVersion uint64 `json:"-"`
	// BillingTenantID is set only by the authenticated HTTP boundary after its
	// subscription check. It is intentionally excluded from the persisted
	// request JSON; Task owns the durable billing identity separately from the
	// caller-facing ownership tenant.
	BillingTenantID    string           `json:"-"`
	UserID             string           `json:"user_id,omitempty"`
	Text               string           `json:"text,omitempty"`
	Source             *SourceReference `json:"source,omitempty"`
	Platforms          []string         `json:"platforms,omitempty"`
	Country            string           `json:"country,omitempty"`
	Language           string           `json:"language,omitempty"`
	SheinStoreID       int64            `json:"shein_store_id,omitempty"`
	TargetCategoryHint string           `json:"target_category_hint,omitempty"`
	BrandHint          string           `json:"brand_hint,omitempty"`
	Options            *GenerateOptions `json:"options,omitempty"`
}

type WarmSDSBaselineRequest struct {
	TenantID string          `json:"tenant_id,omitempty"`
	SDS      *SDSSyncOptions `json:"sds,omitempty"`
}

type GenerateOptions struct {
	SDS *SDSSyncOptions `json:"sds,omitempty"`
}
