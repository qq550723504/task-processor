package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type FieldError = sheinworkspace.FieldError

func ValidateRevisionInput(req *RevisionInput) []FieldError {
	return sheinworkspace.ValidateRevisionInput(req)
}

func NewFieldError(fieldPath, code, message string) FieldError {
	return sheinworkspace.NewFieldError(fieldPath, code, message)
}
