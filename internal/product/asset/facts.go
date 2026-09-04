package asset

// Facts is the provider-neutral source-asset handoff shape.
type Facts struct {
	SourceKey      string
	SourceType     string
	SourcePlatform string
	SourceID       string
	Items          []ItemFacts
	Warnings       []FactWarning
}

type ItemFacts struct {
	SourceID  string
	URL       string
	MediaType string
	Role      string
	Checksum  string
}

type FactWarning struct {
	Code    string
	Message string
	Field   string
}

func (f Facts) HasAssets() bool {
	return len(f.Items) > 0
}
