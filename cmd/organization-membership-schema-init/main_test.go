package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRunRequiresAnExplicitDSNFile(t *testing.T) {
	var output strings.Builder
	if got := run(nil, &output, func(context.Context, string) error { t.Fatal("initializer called"); return nil }); got == 0 {
		t.Fatal("missing dsn file accepted")
	}
}

func TestRunPassesPrivateDSNToInitializer(t *testing.T) {
	path := t.TempDir() + "\\membership-dsn"
	if err := os.WriteFile(path, []byte("postgresql://owner:password@127.0.0.1:5436/membership?sslmode=disable\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var got string
	if code := run([]string{"-dsn-file", path, "-timeout", "2s"}, new(strings.Builder), func(_ context.Context, dsn string) error { got = dsn; return nil }); code != 0 {
		t.Fatalf("run code = %d", code)
	}
	if got != "postgresql://owner:password@127.0.0.1:5436/membership?sslmode=disable" {
		t.Fatalf("dsn = %q", got)
	}
}
