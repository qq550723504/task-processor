package listingkit

import (
	amazonlisting "task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
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
	visualAssetBundle *asset.Bundle
	renderPreviews    *PlatformAssetRenderPreviews
	needsReview       bool
	summary           []string
	readiness         *SheinSubmitReadiness
	checklist         *SheinSubmitChecklist
	repairCenter      *SheinRepairCenter
	statusOverview    *sheinworkspace.StatusOverview
	workspaceOverview *sheinworkspace.WorkspaceOverview
}
