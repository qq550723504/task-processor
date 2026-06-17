package attribute

import (
	"context"
)

type PlatformValueFallbackRequest struct {
	AttrID         int
	Domain         platformValueDomain
	RawValue       string
	ProductTitle   string
	PlatformValues []string
	SizeChart      string
}

type PlatformValueFallbackResult struct {
	ResolvedValue string  `json:"resolved_value"`
	Confidence    float64 `json:"confidence"`
	Reason        string  `json:"reason"`
}

type platformValueFallbackResolver interface {
	ResolvePlatformValue(ctx context.Context, req *PlatformValueFallbackRequest) (*PlatformValueFallbackResult, error)
}
