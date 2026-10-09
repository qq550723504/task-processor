package target

import (
	"errors"
	"time"
)

const Timeout = 10 * time.Second
const MaxPayloadBytes = 2 << 20

var (
	ErrInvalid     = errors.New("invalid listing target request")
	ErrForbidden   = errors.New("listing target permission denied")
	ErrNotFound    = errors.New("listing target resource unavailable")
	ErrNotReady    = errors.New("listing target exact inputs are not ready")
	ErrConflict    = errors.New("listing target operation conflict")
	ErrUnavailable = errors.New("listing target dependency unavailable")
	ErrTooLarge    = errors.New("listing target payload exceeds limit")
)
