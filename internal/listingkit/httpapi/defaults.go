package httpapi

func ResolveDefaultSheinStoreID(storeIDs []int64) int64 {
	if len(storeIDs) != 1 {
		return 0
	}
	if storeIDs[0] <= 0 {
		return 0
	}
	return storeIDs[0]
}
