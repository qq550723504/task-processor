package preview

// ReadModelInput captures the result-driven preview data that legacy adapters
// can project without owning preview composition details themselves.
type ReadModelInput struct {
	NeedsReview         bool
	Attachment          *AttachmentInput
	Overview            *HeaderInput
	RevisionHistoryMeta *RevisionHistoryMetaInput
}

func BuildReadModel(input ReadModelInput) *Preview {
	return buildPreviewWithReadModel(BuildShell(ShellInput{}), input)
}
