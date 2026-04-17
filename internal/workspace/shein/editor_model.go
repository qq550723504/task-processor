package shein

type EditorRecommendationMeta struct {
	Source      string   `json:"source,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	ReviewNotes []string `json:"review_notes,omitempty"`
}

type EditorAttributeSuggestion struct {
	Name             string `json:"name,omitempty"`
	Value            string `json:"value,omitempty"`
	AttributeID      int    `json:"attribute_id,omitempty"`
	AttributeValueID *int   `json:"attribute_value_id,omitempty"`
	Source           string `json:"source,omitempty"`
	Confidence       string `json:"confidence,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type EditorSaleCandidateSuggestion struct {
	Name           string   `json:"name,omitempty"`
	AttributeID    int      `json:"attribute_id,omitempty"`
	SelectedScope  string   `json:"selected_scope,omitempty"`
	SampleValue    string   `json:"sample_value,omitempty"`
	PrimaryScore   int      `json:"primary_score,omitempty"`
	SecondaryScore int      `json:"secondary_score,omitempty"`
	Source         string   `json:"source,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
}

type EditorEffect struct {
	Key            string   `json:"key,omitempty"`
	Label          string   `json:"label,omitempty"`
	AffectedFields []string `json:"affected_fields,omitempty"`
	PreviewBlocks  []string `json:"preview_blocks,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type EditorProgress struct {
	Completed  int                     `json:"completed"`
	Total      int                     `json:"total"`
	Unresolved int                     `json:"unresolved"`
	Sections   []EditorProgressSection `json:"sections,omitempty"`
}

type EditorProgressSection struct {
	Key        string `json:"key,omitempty"`
	Label      string `json:"label,omitempty"`
	Completed  int    `json:"completed"`
	Total      int    `json:"total"`
	Unresolved int    `json:"unresolved"`
	Status     string `json:"status,omitempty"`
}
