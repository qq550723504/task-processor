package listingkit

import (
	amazonlisting "task-processor/internal/amazonlisting"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

type amazonPreviewPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualPreviewPayloadBase
}

type reviewablePlatformPreviewPayloadInput struct {
	base reviewablePlatformPreviewPayloadBase
}

type amazonExportPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualExportBase
}

type reviewableExportPayloadInput struct {
	visualBase platformVisualExportBase
}

type sheinPreviewPayloadInput struct {
	pkg               *sheinpub.Package
	canonical         *canonical.Product
	needsReview       bool
	summary           []string
	readiness         *SheinSubmitReadiness
	checklist         *SheinSubmitChecklist
	repairCenter      *SheinRepairCenter
	statusOverview    *sheinworkspace.StatusOverview
	workspaceOverview *sheinworkspace.WorkspaceOverview
}
