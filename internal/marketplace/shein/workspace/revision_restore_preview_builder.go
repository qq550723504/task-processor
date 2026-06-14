package workspace

func BuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	context *Ctx,
	safety *Safety,
	compare *Compare,
	presentation *Pres,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	if draft == nil && revisionPayload == nil && context == nil && safety == nil && compare == nil && presentation == nil {
		return nil
	}
	return &RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres]{
		Core: &RestorePreviewCoreData[Req, Ctx, Safety, Compare]{
			Draft:           draft,
			RevisionPayload: revisionPayload,
			Context:         context,
			Safety:          safety,
			Compare:         compare,
		},
		Presentation: presentation,
	}
}

func RebuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	src *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres],
	compare *Compare,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	if src == nil {
		return nil
	}
	var draft *EditorRevisionSkeleton
	var revisionPayload *Req
	var context *Ctx
	var safety *Safety
	var presentation *Pres
	if src.Core != nil {
		draft = src.Core.Draft
		revisionPayload = src.Core.RevisionPayload
		context = src.Core.Context
		safety = src.Core.Safety
		if compare == nil {
			compare = src.Core.Compare
		}
	}
	presentation = src.Presentation
	return BuildRestorePreviewPayload(draft, revisionPayload, context, safety, compare, presentation)
}
