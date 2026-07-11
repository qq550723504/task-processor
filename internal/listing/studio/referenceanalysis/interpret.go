package referenceanalysis

import "strings"

func Interpret(rawAnalyses []string) (Result, error) {
	for _, raw := range rawAnalyses {
		if strings.TrimSpace(raw) != "" {
			return Result{}, ErrNoSafeDirection
		}
	}
	return Result{}, ErrNoInput
}
