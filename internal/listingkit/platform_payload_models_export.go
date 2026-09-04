package listingkit

import (
	"task-processor/internal/amazonlisting"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type AmazonExportPayload struct {
	Draft       *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
	ImageBundle *common.PublishImageBundle        `json:"image_bundle,omitempty"`
}

type SheinExportPayload struct {
	Inspection  *sheinpub.Inspection       `json:"inspection,omitempty"`
	ImageBundle *common.PublishImageBundle `json:"image_bundle,omitempty"`
	// Deprecated: kept only for export JSON compatibility. New business code should use DraftPayload.
	RequestDraft *sheinpub.RequestDraft `json:"request_draft,omitempty"`
	// DraftPayload is the canonical SHEIN draft payload exposed to internal export builders.
	DraftPayload *sheinpub.RequestDraft `json:"draft_payload,omitempty"`
	// Deprecated: kept only for export JSON compatibility. New business code should use PreviewPayload.
	PreviewProduct *sheinproduct.Product `json:"preview_product,omitempty"`
	// PreviewPayload is the canonical SHEIN preview payload exposed to internal export builders.
	PreviewPayload *sheinproduct.Product `json:"preview_payload,omitempty"`
	ReviewNotes    []string              `json:"review_notes,omitempty"`
}

type TemuExportPayload struct {
	ImageBundle *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Package     *TemuPackage               `json:"package,omitempty"`
}

type WalmartExportPayload struct {
	ImageBundle *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Package     *WalmartPackage            `json:"package,omitempty"`
}
