package config

import (
	"os"
	"strings"
)

// ProductListingAPIRuntimeAutoMigrateEnabled reports whether runtime database
// migrations are allowed for the product-listing API process. It defaults to
// true for backwards compatibility, while allowing local debugging sessions to
// connect to a shared database without modifying its schema.
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
