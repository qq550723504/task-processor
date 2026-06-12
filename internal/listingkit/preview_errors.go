package listingkit

import (
	"errors"

	previewdomain "task-processor/internal/listing/preview"
)

var ErrUnsupportedPreviewPlatform = previewdomain.ErrUnsupportedPlatform
var ErrPreviewPlatformUnavailable = previewdomain.ErrPlatformUnavailable
var ErrTaskResultUnavailable = previewdomain.ErrResultUnavailable
var ErrInvalidRevisionRequest = errors.New("invalid revision request")
