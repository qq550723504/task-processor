package openmeter

import (
	"context"
	"errors"
	"net"
	"syscall"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

// FailureKind identifies how callers should handle an OpenMeter operation failure.
type FailureKind string

const (
	FailureRetryable     FailureKind = "retryable"
	FailurePermanent     FailureKind = "permanent"
	FailureConfiguration FailureKind = "configuration"
)

var errConfiguration = errors.New("openmeter configuration failure")

// ClassifyError maps SDK and network errors to the adapter's retry policy boundary.
func ClassifyError(err error) FailureKind {
	if errors.Is(err, errConfiguration) {
		return FailureConfiguration
	}

	if apiErr, ok := openmeterapi.AsAPIError(err); ok {
		switch {
		case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
			return FailureConfiguration
		case apiErr.StatusCode == 408 || apiErr.StatusCode == 429 || apiErr.StatusCode >= 500:
			return FailureRetryable
		default:
			return FailurePermanent
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return FailureRetryable
	}

	var networkErr net.Error
	if errors.As(err, &networkErr) && (networkErr.Timeout() || isTemporary(networkErr)) {
		return FailureRetryable
	}

	return FailurePermanent
}

func isTemporary(err error) bool {
	temporary, ok := err.(interface{ Temporary() bool })
	return ok && temporary.Temporary()
}
