package referenceanalysis

import "errors"

var (
	ErrNoInput         = errors.New("reference analysis input is empty")
	ErrNoSafeDirection = errors.New("reference analysis has no reusable safe style direction")
	ErrEmptyPrompt     = errors.New("reference analysis generated an empty prompt")
)

type Result struct {
	StyleBrief        string
	SanitizedPrompt   string
	HadUnsafeInput    bool
	HadMalformedInput bool
}

type imageAnalysis struct {
	Motif            string   `json:"motif,omitempty"`
	Palette          []string `json:"palette,omitempty"`
	Composition      string   `json:"composition,omitempty"`
	Typography       string   `json:"typography,omitempty"`
	Density          string   `json:"density,omitempty"`
	ProductFit       string   `json:"product_fit,omitempty"`
	Mood             string   `json:"mood,omitempty"`
	GarmentPlacement string   `json:"garment_placement,omitempty"`
	Avoid            []string `json:"avoid,omitempty"`
	Raw              string   `json:"-"`
}

type abstractedAnalysis struct {
	Motif            string
	Palette          []string
	Composition      []string
	Typography       string
	Density          string
	ProductFit       string
	Mood             string
	GarmentPlacement string
	HadUnsafe        bool
	HadMalformed     bool
}
