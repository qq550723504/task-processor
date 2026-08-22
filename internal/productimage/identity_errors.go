package productimage

import (
	"errors"

	"task-processor/internal/aicapability"
	"task-processor/internal/shared/aiidentity"
)

// IsIdentityIntegrityError recognizes every provider-neutral identity rejection
// that must fail closed across ProductImage workflow boundaries.
func IsIdentityIntegrityError(err error) bool {
	return errors.Is(err, aiidentity.ErrIdentityIntegrity) ||
		errors.Is(err, aiidentity.ErrMissingIdentity) ||
		aicapability.CategoryOf(err) == aicapability.ErrorIdentityIntegrity
}
