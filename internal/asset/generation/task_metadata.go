package generation

import "strconv"

func cloneTaskMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func reviewConfidenceFromMetadata(metadata map[string]string) float64 {
	if len(metadata) == 0 {
		return 0
	}
	raw := metadata["review_confidence"]
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}
