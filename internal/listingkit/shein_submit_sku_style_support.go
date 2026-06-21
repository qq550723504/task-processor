package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func looksLikeStudioSubmitRequestToken(token string) bool {
	return sheinpub.LooksLikeSubmitRequestToken(token)
}

func looksLikeStudioSubmitTaskToken(token string) bool {
	return sheinpub.LooksLikeSubmitTaskToken(token)
}

func resolveStudioSubmitStyleSuffix(task *Task) string {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return ""
	}
	if value := firstNonEmptyString(
		sheinStudioStyleID(task.Request.Options.SheinStudio),
		task.Request.Options.SDS.StyleID,
	); strings.TrimSpace(value) != "" {
		return value
	}
	return sheinpub.DeriveSubmitStyleSuffix(
		task.Request.Text,
		task.Request.Options.SDS.ProductEnglishName,
		task.Request.Options.SDS.ProductName,
	)
}

func sheinStudioStyleID(options *SheinStudioOptions) string {
	if options == nil {
		return ""
	}
	return options.StyleID
}

func deriveStudioSubmitStyleSuffix(values ...string) string {
	return sheinpub.DeriveSubmitStyleSuffix(values...)
}

func tokenizeStudioStyleSuffixWords(value string) []string {
	return sheinpub.TokenizeStyleSuffixWords(value)
}

func studioSubmitTaskDiscriminator(taskID string) string {
	return sheinpub.SubmitTaskDiscriminator(taskID)
}

func studioSubmitRequestDiscriminator(requestID string) string {
	return sheinpub.SubmitRequestDiscriminator(requestID)
}

func combineStudioSubmitDiscriminators(values ...string) string {
	return sheinpub.CombineSubmitDiscriminators(values...)
}
