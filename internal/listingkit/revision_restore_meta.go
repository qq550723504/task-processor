package listingkit

func revisionRestoreSourceID(req *ApplyRevisionRequest) string {
	if req == nil {
		return ""
	}
	return req.RestoreFromRevisionID
}
