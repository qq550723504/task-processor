package shein

type RestoreRequestSeed struct {
	Platform string
	Actor    string
	Reason   string
	Shein    *RevisionInput
}

func BuildRestoreRequestSeed(draft *EditorRevisionSkeleton) *RestoreRequestSeed {
	if draft == nil {
		return nil
	}
	return &RestoreRequestSeed{
		Platform: draft.Platform,
		Actor:    draft.Actor,
		Reason:   draft.Reason,
		Shein:    CloneRevisionInput(draft.Shein),
	}
}
