package listingscheduler

import (
	"flag"
	"testing"
)

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath(""); got != "config/config-prod.yaml" {
		t.Fatalf("ResolveConfigPath(\"\") = %q", got)
	}
	if got := ResolveConfigPath("config/custom.yaml"); got != "config/custom.yaml" {
		t.Fatalf("ResolveConfigPath(custom) = %q", got)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs, "-config", "config/scheduler.yaml", "-log-level", "debug")
	if opts.Config != "config/scheduler.yaml" {
		t.Fatalf("Config = %q", opts.Config)
	}
	if opts.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", opts.LogLevel)
	}
}
