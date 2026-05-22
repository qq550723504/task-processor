package listingkit

// RetryChildTaskRequest retriggers a specific child task kind.
type RetryChildTaskRequest struct {
	Kind    string         `json:"kind"`
	Options map[string]any `json:"options,omitempty"`
}
