package productimage

import (
	"errors"
	"fmt"
)

var ErrTenantModelAccessDenied = errors.New("tenant model access denied")

func NewTenantModelAccessDeniedError(tenantID string) error {
	return NewNoRetryError(fmt.Errorf("%w for tenant %q", ErrTenantModelAccessDenied, tenantID))
}

func IsTenantModelAccessDenied(err error) bool {
	return errors.Is(err, ErrTenantModelAccessDenied)
}
