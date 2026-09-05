//go:build windows

package ownershipmigration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreflightRejectsDescendantJunctionOverlap(t *testing.T) {
	s, root := fixture(t)
	second := filepath.Join(root, "202", "8")
	if err := os.MkdirAll(second, 0700); err != nil {
		t.Fatal(err)
	}
	shared := supportedTestTempDir(t)
	for _, account := range []string{filepath.Join(root, "101", "7"), second} {
		alias := filepath.Join(account, "Default")
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "New-Item -ItemType Junction -Path $env:OWNERSHIP_TEST_ALIAS -Target $env:OWNERSHIP_TEST_ROOT | Out-Null")
		cmd.Env = append(os.Environ(), "OWNERSHIP_TEST_ALIAS="+alias, "OWNERSHIP_TEST_ROOT="+shared)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v %s", err, output)
		}
		t.Cleanup(func() { _ = os.Remove(alias) })
	}
	s.Accounts = append(s.Accounts, LegacyAccount{ID: 8, TenantID: 202, Platform: "1688", ProfileRef: "second"})
	s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-B", Value: []byte("202")})
	if r, err := Preflight(context.Background(), s, root); err == nil || r.Digest != "" {
		t.Fatal("descendant junction overlap produced a usable receipt")
	}
}

func TestPreflightAcceptsOrdinaryWindowsDescendants(t *testing.T) {
	s, root := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "101", "7", "Default", "Cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if r, err := Preflight(context.Background(), s, root); err != nil || r.Digest == "" {
		t.Fatalf("ordinary profile rejected: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateProfileSubtrees(ctx, nil); err != context.Canceled {
		t.Fatalf("cancelled scan: %v", err)
	}
}

func TestPreflightRejectsCrossAccountWindowsHardLinks(t *testing.T) {
	s, root := fixture(t)
	firstProfile := filepath.Join(root, "101", "7", "Default")
	secondProfile := filepath.Join(root, "202", "8", "Default")
	for _, profile := range []string{firstProfile, secondProfile} {
		if err := os.MkdirAll(profile, 0700); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(firstProfile, "Cookies")
	second := filepath.Join(secondProfile, "Cookies")
	if err := os.WriteFile(first, []byte("shared fixture state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	s.Accounts = append(s.Accounts, LegacyAccount{ID: 8, TenantID: 202, Platform: "1688", ProfileRef: "second"})
	s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-B", Value: []byte("202")})
	if r, err := Preflight(context.Background(), s, root); err == nil || r.Digest != "" {
		t.Fatal("cross-account hard-linked profile state produced usable evidence")
	}
	left, leftErr := os.Stat(first)
	right, rightErr := os.Stat(second)
	if leftErr != nil || rightErr != nil || !os.SameFile(left, right) {
		t.Fatalf("validation modified hard links: %v %v", leftErr, rightErr)
	}
}

func TestPreflightAcceptsSameAccountWindowsHardLinks(t *testing.T) {
	s, root := fixture(t)
	profile := filepath.Join(root, "101", "7", "Default")
	if err := os.MkdirAll(profile, 0700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(profile, "Cookies")
	if err := os.WriteFile(first, []byte("same-account fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, filepath.Join(profile, "Cookies.backup")); err != nil {
		t.Fatal(err)
	}
	if r, err := Preflight(context.Background(), s, root); err != nil || r.Digest == "" {
		t.Fatalf("same-account hard links rejected: %v", err)
	}
}
