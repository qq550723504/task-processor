package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationConditionalSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("generation_conditional_state.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_conditional_state.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func buildGenerationConditionalState(deltaToken string, notModified bool, noChanges bool) *GenerationConditionalState {",
		"func applyGenerationConditionalStateToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {",
		"func applyGenerationConditionalStateToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {",
		"func applyGenerationConditionalStateToNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("generation_conditional_state.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildGenerationDispatchResponseDescriptors(response *GenerationReviewNavigationDispatchResponse) []GenerationPanelResourceDescriptor {",
		"func buildGenerationReviewPanelUpdateFromDispatch(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewPanelUpdate {",
		"func mergeSupplementalGenerationDispatchResultsIntoPanelUpdate(update *GenerationReviewPanelUpdate, response *GenerationReviewNavigationDispatchResponse) {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("generation_conditional_state.go should delegate helper seam %q", needle)
		}
	}

	descriptorSrc, err := os.ReadFile("generation_conditional_descriptor_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_conditional_descriptor_support.go) error = %v", err)
	}
	descriptorContent := string(descriptorSrc)

	for _, needle := range []string{
		"func buildGenerationQueueResponseDescriptors(page *GenerationQueuePage) []GenerationPanelResourceDescriptor {",
		"func buildGenerationDispatchResponseDescriptors(response *GenerationReviewNavigationDispatchResponse) []GenerationPanelResourceDescriptor {",
		"func buildGenerationPanelFocusedDescriptors(update *GenerationReviewPanelUpdate) []GenerationPanelResourceDescriptor {",
		"func uniqueGenerationPanelResourceDescriptors(items []GenerationPanelResourceDescriptor) []GenerationPanelResourceDescriptor {",
	} {
		if !strings.Contains(descriptorContent, needle) {
			t.Fatalf("generation_conditional_descriptor_support.go should contain %q", needle)
		}
	}

	panelSrc, err := os.ReadFile("generation_conditional_panel_update_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_conditional_panel_update_support.go) error = %v", err)
	}
	panelContent := string(panelSrc)

	for _, needle := range []string{
		"func buildGenerationReviewPanelUpdateFromDispatch(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewPanelUpdate {",
		"func mergeSupplementalGenerationDispatchResultsIntoPanelUpdate(update *GenerationReviewPanelUpdate, response *GenerationReviewNavigationDispatchResponse) {",
		"func mergeGenerationReviewSessionIntoPanelUpdate(update *GenerationReviewPanelUpdate, session *GenerationReviewSession) {",
		"func firstReviewPatch(current, candidate *GenerationReviewSessionPatch) *GenerationReviewSessionPatch {",
	} {
		if !strings.Contains(panelContent, needle) {
			t.Fatalf("generation_conditional_panel_update_support.go should contain %q", needle)
		}
	}
}
