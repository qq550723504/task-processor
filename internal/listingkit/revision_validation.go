package listingkit

import (
	"fmt"
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

type RevisionFieldError = sheinworkspace.FieldError

type RevisionValidationError struct {
	Fields []RevisionFieldError `json:"fields,omitempty"`
}

func (e *RevisionValidationError) Error() string {
	if e == nil || len(e.Fields) == 0 {
		return ErrInvalidRevisionRequest.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidRevisionRequest.Error(), e.Fields[0].Message)
}

func (e *RevisionValidationError) Unwrap() error {
	return ErrInvalidRevisionRequest
}

func validateApplyRevisionRequest(req *ApplyRevisionRequest) error {
	if req == nil {
		return ErrInvalidRevisionRequest
	}

	var fieldErrors []RevisionFieldError
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	switch platform {
	case "shein":
		if req.Shein == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "shein",
				Code:      "required",
				Message:   "缺少 shein revision payload",
			})
			break
		}
		fieldErrors = append(fieldErrors, validateSheinRevisionInput(req.Shein)...)
	case "amazon":
		if req.Amazon == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "amazon",
				Code:      "required",
				Message:   "缺少 amazon revision payload",
			})
		}
	case "temu":
		if req.Temu == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "temu",
				Code:      "required",
				Message:   "缺少 temu revision payload",
			})
		}
	case "walmart":
		if req.Walmart == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "walmart",
				Code:      "required",
				Message:   "缺少 walmart revision payload",
			})
		}
	}

	if len(fieldErrors) == 0 {
		return nil
	}
	return &RevisionValidationError{Fields: fieldErrors}
}

func validateSheinRevisionInput(req *SheinRevisionInput) []RevisionFieldError {
	return sheinworkspace.ValidateRevisionInput(req)
}

func newRevisionFieldError(fieldPath, code, message string) RevisionFieldError {
	return sheinworkspace.NewFieldError(fieldPath, code, message)
}
