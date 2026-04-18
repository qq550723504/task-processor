package listingkit

import "slices"

func sortedUniqueStrings(values []string) []string {
	out := uniqueStrings(values)
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}
