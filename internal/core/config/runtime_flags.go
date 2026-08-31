package config

import (
	"os"
	"strings"
)

// ProductListingAPIRuntimeAutoMigrateEnabled is retained only for the legacy
// internal/productimage/httpapi/task_repository_builder.go and
// internal/productenrich/httpapi/bootstrap.go entrypoints. New app composition
// evaluates this switch through OpenFeature instead.
func ProductListingAPIRuntimeAutoMigrateEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return true
	}
}
