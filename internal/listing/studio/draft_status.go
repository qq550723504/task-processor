package studio

// DraftStatus is the persisted lifecycle status for a studio batch draft.
type DraftStatus string

const (
	DraftStatusSelecting  DraftStatus = "selecting"
	DraftStatusGenerating DraftStatus = "generating"
	DraftStatusReviewing  DraftStatus = "reviewing"
)

type DraftStatusInput struct {
	GenerationJobCount int
	DesignCount        int
}

// ResolveDraftStatus derives the draft lifecycle from the materialized
// artifacts in precedence order. Generation state must win over older designs
// and created tasks because it represents the active operation.
func ResolveDraftStatus(input DraftStatusInput) DraftStatus {
	switch {
	case input.GenerationJobCount > 0:
		return DraftStatusGenerating
	case input.DesignCount > 0:
		return DraftStatusReviewing
	default:
		return DraftStatusSelecting
	}
}
