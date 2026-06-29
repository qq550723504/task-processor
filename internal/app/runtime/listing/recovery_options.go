package listing

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

const defaultPausedTaskRecoveryReason = "STORE_DISPATCH_DISABLED"

type PausedTaskRecoveryOptions struct {
	Platform           string
	Config             string
	LogLevel           string
	Execute            bool
	AllowedReasonCodes []string
	StoreIDs           []int64
	Version            string
	BuildTime          string
}

func (o PausedTaskRecoveryOptions) ConfigPath() string {
	return ResolveConfigPath(o.Config)
}

func ParsePausedTaskRecoveryFlags(platform string, args []string) (PausedTaskRecoveryOptions, error) {
	opts := PausedTaskRecoveryOptions{
		Platform: platform,
		Config:   defaultConfigPath,
		LogLevel: "info",
		Execute:  false,
	}
	var reasons string
	var stores string
	fs := flag.NewFlagSet("recover-paused-tasks", flag.ContinueOnError)
	fs.StringVar(&opts.Config, "config", defaultConfigPath, "config path")
	fs.StringVar(&opts.LogLevel, "log-level", "info", "log level")
	fs.BoolVar(&opts.Execute, "execute", false, "execute recovery instead of dry-run")
	fs.StringVar(&reasons, "reason", defaultPausedTaskRecoveryReason, "comma-separated allowed pause reason codes")
	fs.StringVar(&stores, "store", "", "comma-separated listing_store ids to include")
	if err := fs.Parse(args); err != nil {
		return PausedTaskRecoveryOptions{}, err
	}
	opts.AllowedReasonCodes = splitReasonCodes(reasons)
	storeIDs, err := splitStoreIDs(stores)
	if err != nil {
		return PausedTaskRecoveryOptions{}, err
	}
	opts.StoreIDs = storeIDs
	return opts, nil
}

func splitReasonCodes(raw string) []string {
	parts := strings.Split(raw, ",")
	reasons := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			reasons = append(reasons, part)
		}
	}
	return reasons
}

func splitStoreIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	storeIDs := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		storeID, err := strconv.ParseInt(part, 10, 64)
		if err != nil || storeID <= 0 {
			return nil, fmt.Errorf("invalid store id %q", part)
		}
		storeIDs = append(storeIDs, storeID)
	}
	return storeIDs, nil
}
