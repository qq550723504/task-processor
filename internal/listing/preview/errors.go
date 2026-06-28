package preview

import (
	"errors"

	listingplatform "task-processor/internal/listing/platform"
)

var ErrUnsupportedPlatform = listingplatform.ErrUnsupportedPlatform
var ErrPlatformUnavailable = listingplatform.ErrPlatformUnavailable
var ErrResultUnavailable = errors.New("task result unavailable")
