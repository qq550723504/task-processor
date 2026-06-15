package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationReviewSessionSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("generation_review_session_sections.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_review_session_sections.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func buildGenerationReviewSession(result *ListingKitResult, queue *GenerationWorkQueue, query *GenerationQueueQuery) *GenerationReviewSession {",
		"func focusedPreviewAssetID(preview *AssetRenderPreviewSlot) string {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("generation_review_session_sections.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildGenerationReviewSections(queue *GenerationWorkQueue, selectedPlatform, focusCapability string, previews []PlatformAssetRenderPreviews, reviewState *generationReviewState) []GenerationReviewSection {",
		"func buildGenerationReviewSlots(queue *GenerationWorkQueue, selectedPlatform string, previews []PlatformAssetRenderPreviews) []GenerationReviewSlot {",
		"func detectReviewSessionCapability(query *GenerationQueueQuery, slots []GenerationReviewSlot, previews []PlatformAssetRenderPreviews, state *generationReviewState) string {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("generation_review_session_sections.go should delegate helper seam %q", needle)
		}
	}

	sectionSrc, err := os.ReadFile("generation_review_section_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_review_section_support.go) error = %v", err)
	}
	sectionContent := string(sectionSrc)

	for _, needle := range []string{
		"type generationReviewSectionSpec struct {",
		"func buildGenerationReviewSections(queue *GenerationWorkQueue, selectedPlatform, focusCapability string, previews []PlatformAssetRenderPreviews, reviewState *generationReviewState) []GenerationReviewSection {",
		"func applyReviewStateToSection(section *GenerationReviewSection, state *generationReviewState) {",
		"func attachReviewTargetsToSections(sections []GenerationReviewSection) {",
	} {
		if !strings.Contains(sectionContent, needle) {
			t.Fatalf("generation_review_section_support.go should contain %q", needle)
		}
	}

	slotSrc, err := os.ReadFile("generation_review_session_slot_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_review_session_slot_support.go) error = %v", err)
	}
	slotContent := string(slotSrc)

	for _, needle := range []string{
		"func buildGenerationReviewSlots(queue *GenerationWorkQueue, selectedPlatform string, previews []PlatformAssetRenderPreviews) []GenerationReviewSlot {",
		"func detectReviewSessionPlatform(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews) string {",
		"func detectReviewSessionCapability(query *GenerationQueueQuery, slots []GenerationReviewSlot, previews []PlatformAssetRenderPreviews, state *generationReviewState) string {",
		"func enrichReviewTargetsWithContext(slots []GenerationReviewSlot, sections []GenerationReviewSection, cards []ListingKitPlatformCard, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey string, focusedPreview *AssetRenderPreviewSlot) {",
	} {
		if !strings.Contains(slotContent, needle) {
			t.Fatalf("generation_review_session_slot_support.go should contain %q", needle)
		}
	}
}
