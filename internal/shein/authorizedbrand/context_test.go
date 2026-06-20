package authorizedbrand

import (
	"context"
	"testing"

	"task-processor/internal/listingruntime"
)

func TestConfigFromStore_UsesTrimmedEnabledFields(t *testing.T) {
	enabled := true
	cfg := ConfigFromStore(&listingruntime.StoreInfo{
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "  2fd1n  ",
		AuthorizedBrandName:      "  Logitech  ",
	})

	if !cfg.Enabled {
		t.Fatal("ConfigFromStore().Enabled = false, want true")
	}
	if cfg.Code != "2fd1n" {
		t.Fatalf("ConfigFromStore().Code = %q, want 2fd1n", cfg.Code)
	}
	if cfg.Name != "Logitech" {
		t.Fatalf("ConfigFromStore().Name = %q, want Logitech", cfg.Name)
	}
}

func TestConfigFromStore_DisablesWhenSwitchOff(t *testing.T) {
	cfg := ConfigFromStore(&listingruntime.StoreInfo{
		AuthorizedBrandCode: "2fd1n",
		AuthorizedBrandName: "Logitech",
	})

	if cfg.Enabled {
		t.Fatalf("ConfigFromStore().Enabled = true, want false: %+v", cfg)
	}
	if cfg.Code != "" || cfg.Name != "" {
		t.Fatalf("ConfigFromStore() = %+v, want zero-value config", cfg)
	}
}

func TestWithResolvedFromContext_PreservesEnabledValue(t *testing.T) {
	resolved := &Resolved{
		Enabled: true,
		Code:    "2fd1n",
		Name:    "Logitech罗技",
		NameEn:  "Logitech",
	}

	ctx := WithResolved(context.Background(), resolved)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if got == nil {
		t.Fatal("FromContext() returned nil")
	}
	if *got != *resolved {
		t.Fatalf("FromContext() = %+v, want %+v", got, resolved)
	}
}

func TestWithResolvedFromContext_DropsDisabledValue(t *testing.T) {
	ctx := WithResolved(context.Background(), &Resolved{Code: "2fd1n"})
	got, ok := FromContext(ctx)
	if ok || got != nil {
		t.Fatalf("FromContext() = (%+v, %v), want (nil, false)", got, ok)
	}
}
