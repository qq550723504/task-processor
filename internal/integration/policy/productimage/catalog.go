package productimage

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"

	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"

	"gopkg.in/yaml.v3"
)

const (
	catalogSchema           = "product-image-policy/v1"
	maxCatalogDocumentBytes = 8 << 20
)

var ErrInvalidCatalog = errors.New("product image policy catalog is invalid")

//go:embed policies.yaml
var embeddedCatalog []byte

type catalogDocument struct {
	Schema   string          `yaml:"schema"`
	Policies []catalogPolicy `yaml:"policies"`
}

type catalogPolicy struct {
	Marketplace   string               `yaml:"marketplace"`
	Country       string               `yaml:"country"`
	Family        string               `yaml:"family"`
	SceneCategory string               `yaml:"scene_category"`
	Thresholds    catalogThresholds    `yaml:"thresholds"`
	SceneDefaults catalogSceneDefaults `yaml:"scene_defaults"`
}

type catalogThresholds struct {
	MainReview            *float64 `yaml:"main_review"`
	WhiteBackgroundReview *float64 `yaml:"white_background_review"`
	WhiteCanvasPenalty    *float64 `yaml:"white_canvas_penalty"`
}

type catalogSceneDefaults struct {
	SceneCategory     string   `yaml:"scene_category"`
	SceneStyle        string   `yaml:"scene_style"`
	BackgroundTone    string   `yaml:"background_tone"`
	Composition       string   `yaml:"composition"`
	PropsLevel        string   `yaml:"props_level"`
	AudienceHint      string   `yaml:"audience_hint"`
	CustomSceneHint   string   `yaml:"custom_scene_hint"`
	SlotRole          string   `yaml:"slot_role"`
	SlotBrief         string   `yaml:"slot_brief"`
	StyleReferenceIDs []string `yaml:"style_reference_ids"`
}

func LoadEmbedded() (imagepolicy.PolicySet, error) {
	return Decode(bytes.NewReader(embeddedCatalog))
}

func Decode(reader io.Reader) (imagepolicy.PolicySet, error) {
	if reader == nil {
		return imagepolicy.PolicySet{}, ErrInvalidCatalog
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxCatalogDocumentBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxCatalogDocumentBytes {
		return imagepolicy.PolicySet{}, catalogError("read bounded catalog", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var document catalogDocument
	if err := decoder.Decode(&document); err != nil {
		return imagepolicy.PolicySet{}, catalogError("decode strict catalog", err)
	}
	var additional any
	if err := decoder.Decode(&additional); !errors.Is(err, io.EOF) {
		return imagepolicy.PolicySet{}, catalogError("catalog must contain exactly one document", err)
	}
	if document.Schema != catalogSchema || len(document.Policies) == 0 {
		return imagepolicy.PolicySet{}, ErrInvalidCatalog
	}

	set := imagepolicy.PolicySet{Version: document.Schema, Policies: make([]imagepolicy.Policy, len(document.Policies))}
	for index, policy := range document.Policies {
		if policy.Thresholds.MainReview == nil || policy.Thresholds.WhiteBackgroundReview == nil || policy.Thresholds.WhiteCanvasPenalty == nil {
			return imagepolicy.PolicySet{}, ErrInvalidCatalog
		}
		set.Policies[index] = imagepolicy.Policy{
			Key: imagepolicy.PolicyKey{
				Marketplace:   policy.Marketplace,
				Country:       policy.Country,
				Family:        policy.Family,
				SceneCategory: policy.SceneCategory,
			},
			Thresholds: imagepolicy.Thresholds{
				MainReview:            *policy.Thresholds.MainReview,
				WhiteBackgroundReview: *policy.Thresholds.WhiteBackgroundReview,
				WhiteCanvasPenalty:    *policy.Thresholds.WhiteCanvasPenalty,
			},
			SceneDefaults: productimage.SceneOptions{
				SceneCategory:     policy.SceneDefaults.SceneCategory,
				SceneStyle:        policy.SceneDefaults.SceneStyle,
				BackgroundTone:    policy.SceneDefaults.BackgroundTone,
				Composition:       policy.SceneDefaults.Composition,
				PropsLevel:        policy.SceneDefaults.PropsLevel,
				AudienceHint:      policy.SceneDefaults.AudienceHint,
				CustomSceneHint:   policy.SceneDefaults.CustomSceneHint,
				SlotRole:          policy.SceneDefaults.SlotRole,
				SlotBrief:         policy.SceneDefaults.SlotBrief,
				StyleReferenceIDs: append([]string(nil), policy.SceneDefaults.StyleReferenceIDs...),
			},
		}
	}
	if _, err := imagepolicy.NewResolver(set); err != nil {
		return imagepolicy.PolicySet{}, catalogError("validate policy set", err)
	}
	return set, nil
}

func catalogError(operation string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", ErrInvalidCatalog, operation)
	}
	return fmt.Errorf("%w: %s: %v", ErrInvalidCatalog, operation, err)
}
