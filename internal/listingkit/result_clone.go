package listingkit

import "encoding/json"

func cloneListingKitResult(result *ListingKitResult) (*ListingKitResult, error) {
	if result == nil {
		return nil, nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var cloned ListingKitResult
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
