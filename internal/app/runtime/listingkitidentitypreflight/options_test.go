package listingkitidentitypreflight

import (
	"flag"
	"testing"
)

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath(""); got != "config/config-prod.yaml" {
		t.Fatalf("default config path = %q, want config/config-prod.yaml", got)
	}
	if got := ResolveConfigPath("config/custom.yaml"); got != "config/custom.yaml" {
		t.Fatalf("config path = %q, want config/custom.yaml", got)
	}

	fs := flag.NewFlagSet("listingkit-identity-preflight", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs, "-config", "config/runtime.yaml", "-log-level", "debug")
	if opts.Config != "config/runtime.yaml" || opts.LogLevel != "debug" {
		t.Fatalf("parsed options = %+v", opts)
	}
}
