package listingkit

import (
	"encoding/json"
	"testing"
)

func TestGenerateOptionsDoesNotConsumeRetiredSheinStudioInput(t *testing.T) {
	var options GenerateOptions
	if err := json.Unmarshal([]byte(`{"shein_studio":{"style_id":"retired-style"}}`), &options); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	got, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(got) != `{}` {
		t.Fatalf("Marshal() = %s, want retired input to be ignored", got)
	}
}
