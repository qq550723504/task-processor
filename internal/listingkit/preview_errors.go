package listingkit

import "errors"

var ErrUnsupportedPreviewPlatform = errors.New("unsupported preview platform")
var ErrPreviewPlatformUnavailable = errors.New("preview platform unavailable")
var ErrTaskResultUnavailable = errors.New("task result unavailable")
var ErrInvalidRevisionRequest = errors.New("invalid revision request")
