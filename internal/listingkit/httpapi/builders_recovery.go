package httpapi

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListingKitTaskRecoverySweepInterval = 5 * time.Second
	defaultListingKitTaskRecoverySweepLimit    = 20
)

func BuildListingKitTaskRecoverySweepInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_LISTINGKIT_RECOVERY_SWEEP_INTERVAL"))
	if raw == "" {
		return defaultListingKitTaskRecoverySweepInterval
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return defaultListingKitTaskRecoverySweepInterval
	}
	return interval
}

func BuildListingKitTaskRecoverySweepLimit() int {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_LISTINGKIT_RECOVERY_SWEEP_LIMIT"))
	if raw == "" {
		return defaultListingKitTaskRecoverySweepLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultListingKitTaskRecoverySweepLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
