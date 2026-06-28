package listingkit

import (
	"errors"

	listingplatform "task-processor/internal/listing/platform"
	previewdomain "task-processor/internal/listing/preview"
)

var ErrUnsupportedPreviewPlatform = listingplatform.ErrUnsupportedPlatform
var ErrPreviewPlatformUnavailable = listingplatform.ErrPlatformUnavailable
var ErrTaskResultUnavailable = previewdomain.ErrResultUnavailable
var ErrInvalidRevisionRequest = errors.New("invalid revision request")
