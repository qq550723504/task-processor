package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type ValidationPayload[RestorePreview any] = sheinworkspace.ValidationPayload[RestorePreview]

func BuildValidationPayload[RestorePreview any](pkg *Package, restorePreview *RestorePreview) *ValidationPayload[RestorePreview] {
	return sheinworkspace.BuildValidationPayload(pkg, restorePreview)
}
