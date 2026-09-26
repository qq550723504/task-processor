package image

import "errors"

var (
	ErrInputInvalid                  = errors.New("product image input is invalid")
	ErrCapabilityUnsupported         = errors.New("product image capability is unsupported")
	ErrExternalCapabilityUnavailable = errors.New("product image external capability is unavailable")
	ErrOutputValidation              = errors.New("product image output validation failed")
	ErrPolicyRejected                = errors.New("product image policy rejected the operation")
	// Only a trusted Review owner may return this exact value after durably
	// recording a pre-provider rejection and releasing its admission reserve.
	ErrReviewConfirmedNotDispatched = errors.New("product image review was confirmed not dispatched")
)
