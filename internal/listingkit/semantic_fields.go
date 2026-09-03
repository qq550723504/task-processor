package listingkit

import (
	"encoding/json"

	sheinpub "task-processor/internal/publishing/shein"
)

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
	wire, err := cloneListingKitResultForSemanticSerialization(r)
	if err != nil {
		return nil, err
	}
	normalizeListingKitResultSemanticFields(wire)
	return json.Marshal((*alias)(wire))
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
	wire := cloneStandardProductSnapshotForSemanticSerialization(s)
	normalizeStandardProductSnapshotSemanticFields(wire)
	return json.Marshal((*alias)(wire))
}

func cloneListingKitResultForSemanticSerialization(result *ListingKitResult) (*ListingKitResult, error) {
	if result == nil {
		return nil, nil
	}
	wire := *result
	wire.PodExecution = clonePodExecutionSummary(result.PodExecution)
	wire.StandardProductSnapshot = cloneStandardProductSnapshotForSemanticSerialization(result.StandardProductSnapshot)
	if result.Shein != nil {
		var err error
		wire.Shein, err = sheinpub.ClonePackageForPersistence(result.Shein)
		if err != nil {
			return nil, err
		}
	}
	return &wire, nil
}

func cloneStandardProductSnapshotForSemanticSerialization(snapshot *StandardProductSnapshot) *StandardProductSnapshot {
	if snapshot == nil {
		return nil
	}
	wire := *snapshot
	wire.PodExecution = clonePodExecutionSummary(snapshot.PodExecution)
	return &wire
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
