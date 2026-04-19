package listingkit

func filterNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
