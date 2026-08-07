package aicapability

import "strings"

type RoutingMode string

const (
	RoutingModeLegacy RoutingMode = "legacy"
	RoutingModeShadow RoutingMode = "shadow"
	RoutingModeActive RoutingMode = "active"
)

func ParseRoutingMode(value string) (RoutingMode, error) {
	switch RoutingMode(strings.ToLower(strings.TrimSpace(value))) {
	case "", RoutingModeLegacy:
		return RoutingModeLegacy, nil
	case RoutingModeShadow:
		return RoutingModeShadow, nil
	case RoutingModeActive:
		return RoutingModeActive, nil
	default:
		return "", NewError(ErrorInvalidInput, "", nil)
	}
}
