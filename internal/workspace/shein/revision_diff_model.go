package shein

type RevisionDiffPreview struct {
	ChangeCount int                   `json:"change_count"`
	Changes     []RevisionFieldChange `json:"changes,omitempty"`
}

type RevisionFieldChange struct {
	FieldPath string `json:"field_path,omitempty"`
	Label     string `json:"label,omitempty"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}
