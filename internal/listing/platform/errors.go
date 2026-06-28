package platform

import "errors"

var ErrUnsupportedPlatform = errors.New("unsupported preview platform")
var ErrPlatformUnavailable = errors.New("preview platform unavailable")
