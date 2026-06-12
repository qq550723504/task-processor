package preview

import "errors"

var ErrUnsupportedPlatform = errors.New("unsupported preview platform")
var ErrPlatformUnavailable = errors.New("preview platform unavailable")
var ErrResultUnavailable = errors.New("task result unavailable")
