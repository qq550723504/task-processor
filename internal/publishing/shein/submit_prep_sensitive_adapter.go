package shein

import (
	"context"

	"task-processor/internal/listingadmin"
	sheinproduct "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
	"task-processor/internal/shein/submitprep"
)

func CleanSubmitProductSensitiveWords(ctx context.Context, product *sheinproduct.Product) error {
	return submitprep.CleanSensitiveWordsWithContext(ctx, product)
}

func RetrySensitiveWordCleanup(ctx context.Context, product *sheinproduct.Product, validationNotes []string) bool {
	return submitprep.RetrySensitiveWordCleanupWithContext(ctx, product, validationNotes)
}

type sheinSensitiveWordSanitizer interface {
	SanitizeDisplayTextWithContext(ctx *sheinctx.TaskContext, text string) string
}

func newSheinSensitiveWordSanitizer(ctx context.Context) sheinSensitiveWordSanitizer {
	return submitprep.NewSensitiveWordServiceForContext(ctx)
}

func SetSensitiveWordRepository(repo listingadmin.SensitiveWordRepository) func() {
	return submitprep.SetSensitiveWordRepository(repo)
}

func ConfigureSubmitPrepRepositories(
	sensitive listingadmin.SensitiveWordRepository,
	policy listingadmin.GenerationTopicPolicyRepository,
	override listingadmin.GenerationTopicOverrideRepository,
) func() {
	restoreSensitive := submitprep.SetSensitiveWordRepository(sensitive)
	restorePolicy := submitprep.SetGenerationTopicPolicyRepository(policy)
	restoreOverride := submitprep.SetGenerationTopicOverrideRepository(override)
	return func() {
		restoreOverride()
		restorePolicy()
		restoreSensitive()
	}
}
