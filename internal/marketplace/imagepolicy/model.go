package imagepolicy

import (
	"errors"

	productimage "task-processor/internal/product/image"
)

var (
	ErrInvalidPolicySet    = errors.New("product image policy set is invalid")
	ErrInvalidProfileInput = errors.New("product image profile input is invalid")
	ErrPolicyNotFound      = errors.New("product image policy was not found")
)

type PolicyKey struct {
	Marketplace   string
	Country       string
	Family        string
	SceneCategory string
}

type Thresholds struct {
	MainReview            float64
	WhiteBackgroundReview float64
	WhiteCanvasPenalty    float64
}

type Policy struct {
	Key           PolicyKey
	Thresholds    Thresholds
	SceneDefaults productimage.SceneOptions
}

type PolicySet struct {
	Version  string
	Policies []Policy
}

type ProfileInput PolicyKey

type ProductImageProfile struct {
	Key           PolicyKey
	PolicyVersion string
	Thresholds    Thresholds
	SceneDefaults productimage.SceneOptions
}
