package listingkit

type sheinStoreSelection struct {
	Profile          *ListingKitStoreProfile
	Strategy         string
	Reason           string
	MatchedRuleKinds []string
	ManualOverride   bool
	Fallback         bool
}

func sheinStoreResolutionSnapshotFromTask(task *Task) *SheinStoreResolutionSnapshot {
	if task == nil || task.SheinStoreResolutionSnapshot == nil || task.SheinStoreResolutionSnapshot.StoreID <= 0 {
		return nil
	}
	return task.SheinStoreResolutionSnapshot
}

func selectionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinStoreSelection {
	if snapshot == nil || snapshot.StoreID <= 0 {
		return nil
	}
	return &sheinStoreSelection{
		Profile: &ListingKitStoreProfile{
			ID:                snapshot.MatchedProfileID,
			StoreID:           snapshot.StoreID,
			Enabled:           true,
			Site:              snapshot.Site,
			WarehouseCode:     snapshot.WarehouseCode,
			DefaultStock:      snapshot.DefaultStock,
			DefaultSubmitMode: snapshot.DefaultSubmitMode,
			Pricing:           snapshot.Pricing,
		},
		Strategy:         snapshot.Strategy,
		Reason:           snapshot.Reason,
		MatchedRuleKinds: append([]string(nil), snapshot.MatchedRuleKinds...),
		ManualOverride:   snapshot.ManualOverride,
		Fallback:         snapshot.Fallback,
	}
}
