package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type FieldError = sheinmarketplace.FieldError

func ValidateRevisionInput(req *RevisionInput) []FieldError {
	return sheinmarketplace.ValidateRevisionInput(req)
}

func NewFieldError(fieldPath, code, message string) FieldError {
	return sheinmarketplace.NewFieldError(fieldPath, code, message)
}
