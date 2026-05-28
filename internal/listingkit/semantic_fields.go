package listingkit

import "encoding/json"

func normalizeListingKitResultSemanticFields(result *ListingKitResult) *ListingKitResult {
	if result == nil {
		return nil
	}
	if result.SDSDesignResult == nil {
		result.SDSDesignResult = result.SDSSync
	}
	result.SDSSync = result.SDSDesignResult
	result.PodExecution = normalizePodExecutionSummary(result.PodExecution)
	result.StandardProductSnapshot = normalizeStandardProductSnapshotSemanticFields(result.StandardProductSnapshot)
	return result
}

func normalizeStandardProductSnapshotSemanticFields(snapshot *StandardProductSnapshot) *StandardProductSnapshot {
	if snapshot == nil {
		return nil
	}
	if snapshot.SDSDesignResult == nil {
		snapshot.SDSDesignResult = snapshot.SDSSync
	}
	snapshot.SDSSync = snapshot.SDSDesignResult
	snapshot.PodExecution = normalizePodExecutionSummary(snapshot.PodExecution)
	return snapshot
}

func (r *ListingKitResult) MarshalJSON() ([]byte, error) {
	type alias ListingKitResult
	normalizeListingKitResultSemanticFields(r)
	return json.Marshal((*alias)(r))
}

func (r *ListingKitResult) UnmarshalJSON(data []byte) error {
	type alias ListingKitResult
	aux := (*alias)(r)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	normalizeListingKitResultSemanticFields(r)
	return nil
}

func (s *StandardProductSnapshot) MarshalJSON() ([]byte, error) {
	type alias StandardProductSnapshot
	normalizeStandardProductSnapshotSemanticFields(s)
	return json.Marshal((*alias)(s))
}

func (s *StandardProductSnapshot) UnmarshalJSON(data []byte) error {
	type alias StandardProductSnapshot
	aux := (*alias)(s)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	normalizeStandardProductSnapshotSemanticFields(s)
	return nil
}
