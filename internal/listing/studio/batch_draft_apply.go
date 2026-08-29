package studio

import "strings"

// BatchDraftInput is the transport-neutral subset of a studio batch request
// that determines a persisted draft's editable fields. It deliberately omits
// persistence identity, ownership, timestamps, and runtime job metadata.
type BatchDraftInput[ProductImagePrompt any, SelectedSDSImage any, GroupedSelection any, CreatedTask any, GenerationJob any] struct {
	Selection                  SelectionKeyInput
	SelectionDesignType        string
	Prompt                     string
	PromptMode                 string
	StyleCount                 string
	VariationIntensity         string
	ProductImageCount          string
	ProductImagePrompt         string
	ProductImagePrompts        []ProductImagePrompt
	ArtworkModel               string
	ImageStrategy              string
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
	CreatedTasks               []CreatedTask
	GenerationJobs             []GenerationJob
	DesignCount                int
}

// BatchDraftFields contains the normalized fields to apply to a persisted
// batch draft. It is a domain value object, not a persistence model.
type BatchDraftFields[ProductImagePrompt any, SelectedSDSImage any, GroupedSelection any, CreatedTask any, GenerationJob any] struct {
	Selection                  SelectionKeyInput
	SelectionKey               string
	SelectionDesignType        string
	Status                     DraftStatus
	Prompt                     string
	PromptMode                 string
	StyleCount                 string
	VariationIntensity         string
	ProductImageCount          string
	ProductImagePrompt         string
	ProductImagePrompts        []ProductImagePrompt
	ArtworkModel               string
	ImageStrategy              string
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
	CreatedTasks               []CreatedTask
	CreatedTaskIDs             []string
	GenerationJobs             []GenerationJob
}

// ApplyBatchDraftFields normalizes a batch draft request and returns the
// complete editable field set. Callers retain their own transport and
// persistence models, then adapt this value object at their boundary.
func ApplyBatchDraftFields[ProductImagePrompt any, SelectedSDSImage any, GroupedSelection any, CreatedTask any, GenerationJob any](
	input BatchDraftInput[ProductImagePrompt, SelectedSDSImage, GroupedSelection, CreatedTask, GenerationJob],
	isCreate bool,
	createdTaskID func(CreatedTask) string,
) BatchDraftFields[ProductImagePrompt, SelectedSDSImage, GroupedSelection, CreatedTask, GenerationJob] {
	generationJobs := append([]GenerationJob(nil), input.GenerationJobs...)
	if ShouldDropCreateGenerationJobs(isCreate, len(generationJobs)) {
		generationJobs = nil
	}
	legacyTransparentBackground := input.TransparentBackground
	transparencyMode := NormalizeTransparencyMode(input.TransparentBackgroundMode, &legacyTransparentBackground)
	createdTaskIDs := make([]string, 0, len(input.CreatedTasks))
	if createdTaskID != nil {
		for _, task := range input.CreatedTasks {
			if id := createdTaskID(task); strings.TrimSpace(id) != "" {
				createdTaskIDs = append(createdTaskIDs, id)
			}
		}
	}
	selection := cloneSelectionKeyInput(input.Selection)
	return BatchDraftFields[ProductImagePrompt, SelectedSDSImage, GroupedSelection, CreatedTask, GenerationJob]{
		Selection:                  selection,
		SelectionKey:               BuildSelectionKey(selection),
		SelectionDesignType:        NormalizeBatchDesignType(input.SelectionDesignType),
		Status:                     ResolveDraftStatus(DraftStatusInput{GenerationJobCount: len(generationJobs), CreatedTaskCount: len(input.CreatedTasks), DesignCount: input.DesignCount}),
		Prompt:                     input.Prompt,
		PromptMode:                 strings.TrimSpace(input.PromptMode),
		StyleCount:                 input.StyleCount,
		VariationIntensity:         input.VariationIntensity,
		ProductImageCount:          input.ProductImageCount,
		ProductImagePrompt:         input.ProductImagePrompt,
		ProductImagePrompts:        append([]ProductImagePrompt(nil), input.ProductImagePrompts...),
		ArtworkModel:               input.ArtworkModel,
		ImageStrategy:              input.ImageStrategy,
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
		CreatedTasks:               append([]CreatedTask(nil), input.CreatedTasks...),
		CreatedTaskIDs:             createdTaskIDs,
		GenerationJobs:             generationJobs,
	}
}

func cloneSelectionKeyInput(input SelectionKeyInput) SelectionKeyInput {
	input.SelectedVariantIDs = append([]int64(nil), input.SelectedVariantIDs...)
	return input
}
