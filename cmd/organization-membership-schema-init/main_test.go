package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"task-processor/internal/authz"
	"testing"
)

func TestRunRequiresAnExplicitDSNFile(t *testing.T) {
	var output strings.Builder
	if got := run(nil, &output, func(context.Context, string, *slotInventory) error { t.Fatal("initializer called"); return nil }); got == 0 {
		t.Fatal("missing dsn file accepted")
	}
}

func TestRunPassesPrivateDSNToInitializer(t *testing.T) {
	path := t.TempDir() + "\\membership-dsn"
	if err := os.WriteFile(path, []byte("postgresql://owner:password@127.0.0.1:5436/membership?sslmode=disable\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var got string
	if code := run([]string{"-dsn-file", path, "-timeout", "2s"}, new(strings.Builder), func(_ context.Context, dsn string, _ *slotInventory) error { got = dsn; return nil }); code != 0 {
		t.Fatalf("run code = %d", code)
	}
	if got != "postgresql://owner:password@127.0.0.1:5436/membership?sslmode=disable" {
		t.Fatalf("dsn = %q", got)
	}
}

func TestInventoryAcceptsOnlyCompleteExactOrganizationSlots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slots.json")
	var inventory slotInventory
	inventory.ProjectID = "project"
	inventory.Organizations = append(inventory.Organizations, struct {
		OrganizationID string   `json:"organizationId"`
		RoleKeys       []string `json:"roleKeys"`
	}{OrganizationID: "enterprise"})
	for slot := 1; slot <= 64; slot++ {
		inventory.Organizations[0].RoleKeys = append(inventory.Organizations[0].RoleKeys, authz.EnterpriseRoleKey("enterprise", slot))
	}
	write := func() {
		data, err := json.Marshal(inventory)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := readInventory(path); err != nil {
		t.Fatal(err)
	}
	inventory.Organizations[0].RoleKeys[0] = authz.EnterpriseRoleKey("foreign", 1)
	write()
	if _, err := readInventory(path); err == nil {
		t.Fatal("foreign native slot accepted")
	}
}
