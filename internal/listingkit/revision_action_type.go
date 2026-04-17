package listingkit

const (
	RevisionActionTypeEdit    = "edit"
	RevisionActionTypeRestore = "restore"
)

func revisionActionType(req *ApplyRevisionRequest) string {
	if revisionRestoreSourceID(req) != "" {
		return RevisionActionTypeRestore
	}
	return RevisionActionTypeEdit
}
