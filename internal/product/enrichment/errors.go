package enrichment

import "errors"

var (
	ErrInputInvalid                  = errors.New("enrichment input invalid")
	ErrEvidenceInsufficient          = errors.New("enrichment evidence insufficient")
	ErrCapabilityUnsupported         = errors.New("enrichment capability unsupported")
	ErrPolicyRejected                = errors.New("enrichment policy rejected")
	ErrExternalCapabilityUnavailable = errors.New("enrichment external capability unavailable")
	ErrOutputValidation              = errors.New("enrichment output validation failed")
)
