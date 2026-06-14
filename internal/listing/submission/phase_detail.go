package submission

type PhaseDetailLabels struct {
	Validate        string
	PrepareProduct  string
	UploadImages    string
	PreValidate     string
	SubmitRemote    string
	SaveDraftRemote string
	PersistResult   string
	ConfirmRemote   string
}

func PhaseDetail(action, phase string, labels PhaseDetailLabels) string {
	switch phase {
	case "validate":
		return labels.Validate
	case "prepare_product":
		return labels.PrepareProduct
	case "upload_images":
		return labels.UploadImages
	case "pre_validate":
		return labels.PreValidate
	case "submit_remote":
		if action == "save_draft" && labels.SaveDraftRemote != "" {
			return labels.SaveDraftRemote
		}
		return labels.SubmitRemote
	case "persist_result":
		return labels.PersistResult
	case "confirm_remote":
		return labels.ConfirmRemote
	default:
		return phase
	}
}
