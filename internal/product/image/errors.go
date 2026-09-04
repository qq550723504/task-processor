package image

import "errors"

var (
	ErrInputInvalid                  = errors.New("product image input is invalid")
	ErrCapabilityUnsupported         = errors.New("product image capability is unsupported")
	ErrExternalCapabilityUnavailable = errors.New("product image external capability is unavailable")
	ErrOutputValidation              = errors.New("product image output validation failed")
	ErrPolicyRejected                = errors.New("product image policy rejected the operation")
)
