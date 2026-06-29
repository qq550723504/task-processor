package listing

import "testing"

func TestParsePausedTaskRecoveryFlagsDefaultsToDryRun(t *testing.T) {
	opts, err := ParsePausedTaskRecoveryFlags("shein", []string{})
	if err != nil {
		t.Fatalf("ParsePausedTaskRecoveryFlags() error = %v", err)
	}

	if opts.Platform != "shein" {
		t.Fatalf("Platform = %q, want shein", opts.Platform)
	}
	if opts.Execute {
		t.Fatalf("Execute = true, want false")
	}
	if opts.Config == "" {
		t.Fatalf("Config = empty, want default config path")
	}
	if len(opts.AllowedReasonCodes) != 1 || opts.AllowedReasonCodes[0] != "STORE_DISPATCH_DISABLED" {
		t.Fatalf("AllowedReasonCodes = %v, want STORE_DISPATCH_DISABLED", opts.AllowedReasonCodes)
	}
}

func TestParsePausedTaskRecoveryFlagsParsesExecuteAndReasons(t *testing.T) {
	opts, err := ParsePausedTaskRecoveryFlags("shein", []string{
		"--execute",
		"--config", "config/custom.yaml",
		"--log-level", "debug",
		"--reason", "STORE_DISPATCH_DISABLED,DAILY_LIMIT_REACHED",
		"--store", "877, 1040",
	})
	if err != nil {
		t.Fatalf("ParsePausedTaskRecoveryFlags() error = %v", err)
	}

	if !opts.Execute {
		t.Fatalf("Execute = false, want true")
	}
	if opts.Config != "config/custom.yaml" {
		t.Fatalf("Config = %q, want custom path", opts.Config)
	}
	if opts.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", opts.LogLevel)
	}
	if got, want := opts.AllowedReasonCodes, []string{"STORE_DISPATCH_DISABLED", "DAILY_LIMIT_REACHED"}; !equalStrings(got, want) {
		t.Fatalf("AllowedReasonCodes = %v, want %v", got, want)
	}
	if got, want := opts.StoreIDs, []int64{877, 1040}; !equalInt64s(got, want) {
		t.Fatalf("StoreIDs = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
