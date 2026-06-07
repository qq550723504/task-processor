package listingkit

const (
	RevisionActionTypeEdit    = "edit"
	RevisionActionTypeRestore = "restore"
)

func revisionRestoreSourceID(req *ApplyRevisionRequest) string {
	if req == nil {
		return ""
	}
	return req.RestoreFromRevisionID
}

func revisionActionType(req *ApplyRevisionRequest) string {
	if revisionRestoreSourceID(req) != "" {
		return RevisionActionTypeRestore
	}
	return RevisionActionTypeEdit
}
