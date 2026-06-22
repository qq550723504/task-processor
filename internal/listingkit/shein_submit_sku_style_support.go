package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

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
