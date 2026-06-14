package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	context *Ctx,
	safety *Safety,
	compare *Compare,
	presentation *Pres,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinmarketplace.BuildRestorePreviewPayload(draft, revisionPayload, context, safety, compare, presentation)
}

func RebuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	src *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres],
	compare *Compare,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinmarketplace.RebuildRestorePreviewPayload(src, compare)
}
