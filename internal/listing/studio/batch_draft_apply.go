package studio

import "strings"

// BatchDraftInput is the transport-neutral subset of a studio batch request
// that determines a persisted draft's editable fields. It deliberately omits
// persistence identity, ownership, timestamps, and runtime job metadata.
type BatchDraftInput[SelectedSDSImage any, GroupedSelection any, GenerationJob any] struct {
	Selection                  SelectionKeyInput
	SelectionDesignType        string
	Prompt                     string
	PromptMode                 string
	StyleCount                 string
	VariationIntensity         string
	ArtworkModel               string
	GroupedImageMode           string
	SelectedSDSImages          []SelectedSDSImage
	GroupedSelections          []GroupedSelection
	TransparentBackground      bool
	TransparentBackgroundMode  string
	RenderSizeImagesWithSDS    bool
	HotStyleReferenceImageURLs []string
	HotStyleReferenceBrief     string
	HotStyleReferencePrompt    string
	SheinStoreID               string
	ApprovedDesignIDs          []string
	GenerationJobs             []GenerationJob
	DesignCount                int
}

// BatchDraftFields contains the normalized fields to apply to a persisted
// batch draft. It is a domain value object, not a persistence model.
type BatchDraftFields[SelectedSDSImage any, GroupedSelection any, GenerationJob any] struct {
	Selection                  SelectionKeyInput
	SelectionKey               string
	SelectionDesignType        string
	Status                     DraftStatus
	Prompt                     string
	PromptMode                 string
	StyleCount                 string
	VariationIntensity         string
	ArtworkModel               string
	GroupedImageMode           string
	SelectedSDSImages          []SelectedSDSImage
	GroupedSelections          []GroupedSelection
	TransparentBackground      bool
	TransparentBackgroundMode  TransparencyMode
	RenderSizeImagesWithSDS    bool
	HotStyleReferenceImageURLs []string
	HotStyleReferenceBrief     string
	HotStyleReferencePrompt    string
	SheinStoreID               string
	ApprovedDesignIDs          []string
	GenerationJobs             []GenerationJob
}

// ApplyBatchDraftFields normalizes a batch draft request and returns the
// complete editable field set. Callers retain their own transport and
// persistence models, then adapt this value object at their boundary.
func ApplyBatchDraftFields[SelectedSDSImage any, GroupedSelection any, GenerationJob any](
	input BatchDraftInput[SelectedSDSImage, GroupedSelection, GenerationJob],
	isCreate bool,
) BatchDraftFields[SelectedSDSImage, GroupedSelection, GenerationJob] {
	generationJobs := append([]GenerationJob(nil), input.GenerationJobs...)
	if ShouldDropCreateGenerationJobs(isCreate, len(generationJobs)) {
		generationJobs = nil
	}
	legacyTransparentBackground := input.TransparentBackground
	transparencyMode := NormalizeTransparencyMode(input.TransparentBackgroundMode, &legacyTransparentBackground)
	selection := cloneSelectionKeyInput(input.Selection)
	return BatchDraftFields[SelectedSDSImage, GroupedSelection, GenerationJob]{
		Selection:                  selection,
		SelectionKey:               BuildSelectionKey(selection),
		SelectionDesignType:        NormalizeBatchDesignType(input.SelectionDesignType),
		Status:                     ResolveDraftStatus(DraftStatusInput{GenerationJobCount: len(generationJobs), DesignCount: input.DesignCount}),
		Prompt:                     input.Prompt,
		PromptMode:                 strings.TrimSpace(input.PromptMode),
		StyleCount:                 input.StyleCount,
		VariationIntensity:         input.VariationIntensity,
		ArtworkModel:               input.ArtworkModel,
		GroupedImageMode:           input.GroupedImageMode,
		SelectedSDSImages:          append([]SelectedSDSImage(nil), input.SelectedSDSImages...),
		GroupedSelections:          append([]GroupedSelection(nil), input.GroupedSelections...),
		TransparentBackground:      transparencyMode != TransparencyModeNone,
		TransparentBackgroundMode:  transparencyMode,
		RenderSizeImagesWithSDS:    input.RenderSizeImagesWithSDS,
		HotStyleReferenceImageURLs: NormalizeHotStyleReferenceImageURLs(input.HotStyleReferenceImageURLs),
		HotStyleReferenceBrief:     strings.TrimSpace(input.HotStyleReferenceBrief),
		HotStyleReferencePrompt:    strings.TrimSpace(input.HotStyleReferencePrompt),
		SheinStoreID:               input.SheinStoreID,
		ApprovedDesignIDs:          append([]string(nil), input.ApprovedDesignIDs...),
		GenerationJobs:             generationJobs,
	}
}

func cloneSelectionKeyInput(input SelectionKeyInput) SelectionKeyInput {
	input.SelectedVariantIDs = append([]int64(nil), input.SelectedVariantIDs...)
	return input
}
