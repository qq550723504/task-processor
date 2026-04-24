package shein

import "strings"

func comparableAttributeSegments(value string) []string {
	segments := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case '/', '\\', '|', ',', ';', '，', '；':
			return true
		default:
			return false
		}
	})
	if len(segments) <= 1 {
		return nil
	}
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		out = append(out, segment)
	}
	return out
}
