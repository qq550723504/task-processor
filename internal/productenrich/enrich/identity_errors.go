package enrich

import (
	"errors"

	"task-processor/internal/aicapability"
	"task-processor/internal/shared/aiidentity"
)

func isIdentityIntegrityError(err error) bool {
	return errors.Is(err, aiidentity.ErrIdentityIntegrity) || errors.Is(err, aiidentity.ErrMissingIdentity) || aicapability.CategoryOf(err) == aicapability.ErrorIdentityIntegrity
}
