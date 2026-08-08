package listingkit

import "errors"

var ErrUnsupportedSubmitPlatform = errors.New("unsupported submit platform")
var ErrSubmitBlocked = errors.New("submit blocked by readiness")

var ErrInvalidSheinResolutionCacheKind = errors.New("invalid shein resolution cache kind")
var ErrInvalidSheinCategorySearchQuery = errors.New("invalid shein category search query")
