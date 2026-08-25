package studio

import "strings"

type TransparencyMode string

const (
	TransparencyModeNone    TransparencyMode = "none"
	TransparencyModeNative  TransparencyMode = "native"
	TransparencyModeRemoval TransparencyMode = "removal"
)

// NormalizeTransparencyMode preserves explicit modes and translates the
// legacy boolean field only when no explicit mode was supplied.
func NormalizeTransparencyMode(mode string, legacy *bool) TransparencyMode {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch TransparencyMode(normalized) {
	case TransparencyModeNone:
		return TransparencyModeNone
	case TransparencyModeNative:
		return TransparencyModeNative
	case TransparencyModeRemoval:
		return TransparencyModeRemoval
	}
	if normalized != "" {
		return TransparencyModeNone
	}
	if legacy != nil && *legacy {
		return TransparencyModeNative
	}
	return TransparencyModeNone
}
