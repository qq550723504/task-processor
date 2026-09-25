package temporal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
)

func TestOrganizationStagedReviewRequiresOriginalGenerationQuote(t *testing.T) {
	original := imageagent.SlotUsageQuote{Operations: []imageagent.SlotUsageOperation{{Name: "extract_subject", Fingerprint: "extract-1"}, {Name: "review", Fingerprint: "review-1"}}}
	matching := imageagent.SlotUsageQuote{Operations: []imageagent.SlotUsageOperation{{Name: "review", Fingerprint: "review-1"}}}
	require.NoError(t, validateOriginalOrganizationReviewQuote(original, matching))

	changed := imageagent.SlotUsageQuote{Operations: []imageagent.SlotUsageOperation{{Name: "review", Fingerprint: "new-review-route"}}}
	require.ErrorIs(t, validateOriginalOrganizationReviewQuote(original, changed), imageagent.ErrRevisionConflict)
	require.ErrorIs(t, validateOriginalOrganizationReviewQuote(imageagent.SlotUsageQuote{}, matching), imageagent.ErrRevisionConflict)
	require.ErrorIs(t, validateOriginalOrganizationReviewQuote(original, imageagent.SlotUsageQuote{}), imageagent.ErrRevisionConflict)
}
