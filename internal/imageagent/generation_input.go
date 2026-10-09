package imageagent

import (
	"crypto/sha256"
	"encoding/hex"
)

func ValidateImageSetExecution(input SlotExecutionInput) error {
	if input.ImageSet == nil || input.Slot.Recipe == nil || input.ImageSet.Target.Platform != input.TargetPlatform || input.ImageSet.MaxPoints < input.Slot.Recipe.Quote.Points || !generationDigest(input.ImageSet.QuoteDigest) {
		return ErrValidation
	}
	// Validate the exact slot against the same recipe contract as plan creation.
	// Set totals are checked on the complete plan by its owner, not by a slot.
	set := CloneImageSetPlan(input.ImageSet)
	plan := Plan{Set: set, Slots: []Slot{input.Slot}}
	set.MaxPoints = input.Slot.Recipe.Quote.Points
	var err error
	set.QuoteDigest, err = ImageSetQuoteDigest(plan)
	if err != nil {
		return err
	}
	return ValidateImageSetPlan(plan)
}

func ImageSlotGenerationInputDigest(input SlotExecutionInput) (string, error) {
	if err := ValidateImageSetExecution(input); err != nil {
		return "", err
	}
	return ImageGenerationInputDigestFromFingerprint(SlotExecutionFingerprint(input)), nil
}

func ImageGenerationInputDigestFromFingerprint(fingerprint string) string {
	return generationHash(struct{ Schema, Fingerprint string }{ImageSetSchema, fingerprint})
}

func ImageSourceBundleDigest(references []ImageSourceObservation) string {
	return generationHash(references)
}

func ValidateImageSourceBytes(recipe *ImageSlotRecipe, references [][]byte, maxBytes int) error {
	if recipe == nil || len(recipe.References) != len(references) || len(references) < 1 || len(references) > 8 || maxBytes <= 0 {
		return ErrValidation
	}
	total := 0
	for i, data := range references {
		if len(data) == 0 || len(data) > maxBytes {
			return ErrValidation
		}
		total += len(data)
		if total > 16<<20 {
			return ErrValidation
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != recipe.References[i].Bytes || hex.EncodeToString(sum[:]) != recipe.References[i].SHA256 {
			return ErrRevisionConflict
		}
	}
	return nil
}
