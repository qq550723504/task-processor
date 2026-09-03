package imageagent

import "encoding/json"

// SlotProjection has its own JSON representation because AssetCandidate is a
// frozen v2 wire type. The v3 durable identity belongs in persisted
// projections, but it must never become part of SlotExecutionResult's v2 JSON
// payload.
func (value SlotProjection) MarshalJSON() ([]byte, error) {
	var candidates []slotProjectionJSONCandidate
	if value.Candidates != nil {
		candidates = make([]slotProjectionJSONCandidate, len(value.Candidates))
	}
	for index, candidate := range value.Candidates {
		candidates[index] = slotProjectionJSONCandidate{
			AssetID:       candidate.AssetID,
			SourceAssetID: candidate.SourceAssetID,
			Width:         candidate.Width,
			Height:        candidate.Height,
			Operations:    append([]string(nil), candidate.Operations...),
		}
		if candidate.DurableAsset.ObjectKey != "" || candidate.DurableAsset.SHA256 != "" {
			identity := candidate.DurableAsset
			candidates[index].DurableAsset = &identity
			continue
		}
		url := candidate.URL
		metadata := candidate.Metadata
		candidates[index].URL = &url
		candidates[index].Metadata = &metadata
	}
	return json.Marshal(slotProjectionJSON{
		Slot:       value.Slot,
		Attempt:    value.Attempt,
		Candidates: candidates,
		ErrorCode:  value.ErrorCode,
	})
}

func (value *SlotProjection) UnmarshalJSON(raw []byte) error {
	var decoded slotProjectionJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	var candidates []AssetCandidate
	if decoded.Candidates != nil {
		candidates = make([]AssetCandidate, len(decoded.Candidates))
	}
	for index, candidate := range decoded.Candidates {
		candidates[index] = AssetCandidate{
			AssetID:       candidate.AssetID,
			SourceAssetID: candidate.SourceAssetID,
			Width:         candidate.Width,
			Height:        candidate.Height,
			Operations:    append([]string(nil), candidate.Operations...),
		}
		if candidate.URL != nil {
			candidates[index].URL = *candidate.URL
		}
		if candidate.Metadata != nil {
			candidates[index].Metadata = *candidate.Metadata
		}
		if candidate.DurableAsset != nil {
			candidates[index].DurableAsset = *candidate.DurableAsset
		}
	}
	*value = SlotProjection{Slot: decoded.Slot, Attempt: decoded.Attempt, Candidates: candidates, ErrorCode: decoded.ErrorCode}
	return nil
}

type slotProjectionJSON struct {
	Slot       Slot
	Attempt    int
	Candidates []slotProjectionJSONCandidate
	ErrorCode  string
}

type slotProjectionJSONCandidate struct {
	AssetID       string
	URL           *string `json:"URL,omitempty"`
	SourceAssetID string
	Metadata      *map[string]string    `json:"Metadata,omitempty"`
	DurableAsset  *DurableAssetIdentity `json:"DurableAsset,omitempty"`
	Width         int                   `json:"Width,omitempty"`
	Height        int                   `json:"Height,omitempty"`
	Operations    []string              `json:"Operations,omitempty"`
}
