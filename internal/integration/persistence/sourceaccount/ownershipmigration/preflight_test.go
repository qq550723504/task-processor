package ownershipmigration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T) (Snapshot, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "101", "7"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "101", "7", "sentinel"), []byte("existing session"), 0600); err != nil {
		t.Fatal(err)
	}
	return Snapshot{
		Accounts: []LegacyAccount{{ID: 7, TenantID: 101, Platform: "1688", ProfileRef: "opaque-old-reference"}},
		Metadata: []OrganizationMetadata{{OrganizationID: "org-A", Value: []byte("101"), Sequence: 42}},
	}, root
}

func TestPreflightRemovedMetadataValidation(t *testing.T) {
	for _, value := range []string{"", "abc", "0101", "+101", " 101", "0", "-1", "9223372036854775808"} {
		t.Run(value, func(t *testing.T) {
			s, root := fixture(t)
			s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "removed", Value: []byte(value), OwnerRemoved: true})
			if r, err := Preflight(context.Background(), s, root); err == nil || r.Digest != "" {
				t.Fatal("malformed removed metadata produced a usable receipt")
			}
		})
	}
	s, root := fixture(t)
	removed := OrganizationMetadata{OrganizationID: "removed", Value: []byte("101"), Sequence: 42, OwnerRemoved: true}
	s.Metadata = append(s.Metadata, removed)
	r, err := Preflight(context.Background(), s, root)
	if err != nil || len(r.Metadata) != 2 || !reflect.DeepEqual(r.Metadata[1], removed) || r.Accounts[0].OrganizationID != "org-A" {
		t.Fatalf("valid tombstone must remain evidence without competing with active owner: %+v %v", r, err)
	}
	s.Metadata = []OrganizationMetadata{removed}
	if _, err := Preflight(context.Background(), s, root); err == nil {
		t.Fatal("removed metadata contributed active ownership")
	}
}

func TestPreflightPreservesProfileAndDisabledDeletedState(t *testing.T) {
	s, root := fixture(t)
	s.Accounts[0].Status, s.Accounts[0].Deleted = 1, 1
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stage != "preflight_only" || r.Version != 1 || len(r.Digest) != 64 {
		t.Fatalf("invalid receipt: %+v", r)
	}
	a := r.Accounts[0]
	if a.OrganizationID != "org-A" || a.ProfileDirectory != filepath.Join(root, "101", "7") || a.Previous.ProfileRef != "opaque-old-reference" || a.Previous.Status != 1 || a.Previous.Deleted != 1 {
		t.Fatalf("ownership/profile changed: %+v", a)
	}
	data, err := os.ReadFile(filepath.Join(a.ProfileDirectory, "sentinel"))
	if err != nil || string(data) != "existing session" {
		t.Fatalf("profile altered: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "org-A")); !os.IsNotExist(err) {
		t.Fatal("must not create organization directory")
	}
}

func TestPreflightMappingFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Snapshot)
	}{
		{"missing", func(s *Snapshot) { s.Metadata = nil }},
		{"numeric equality is not mapping", func(s *Snapshot) { s.Metadata[0].OrganizationID = "101"; s.Metadata[0].Value = []byte("999") }},
		{"ambiguous organizations", func(s *Snapshot) {
			s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-B", Value: []byte("101")})
		}},
		{"removed", func(s *Snapshot) { s.Metadata[0].OwnerRemoved = true }},
		{"duplicate metadata", func(s *Snapshot) { s.Metadata = append(s.Metadata, s.Metadata[0]) }},
		{"removal must not resurrect", func(s *Snapshot) {
			m := s.Metadata[0]
			m.OwnerRemoved = true
			m.Sequence++
			s.Metadata = append(s.Metadata, m)
		}},
		{"invalid metadata", func(s *Snapshot) { s.Metadata[0].Value = []byte("101x") }},
		{"noncanonical metadata", func(s *Snapshot) { s.Metadata[0].Value = []byte("0101") }},
		{"empty org", func(s *Snapshot) { s.Metadata[0].OrganizationID = "" }},
		{"duplicate account", func(s *Snapshot) { s.Accounts = append(s.Accounts, s.Accounts[0]) }},
		{"other platform", func(s *Snapshot) { s.Accounts[0].Platform = "amazon" }},
		{"missing profile ref", func(s *Snapshot) { s.Accounts[0].ProfileRef = "" }},
		{"invalid account", func(s *Snapshot) { s.Accounts[0].ID = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := fixture(t)
			tc.change(&s)
			r, err := Preflight(context.Background(), s, root)
			if err == nil || r.Digest != "" {
				t.Fatalf("expected rejection without usable receipt: %+v %v", r, err)
			}
		})
	}
}

func TestPreflightRestartIsDeterministicAndBindsMapping(t *testing.T) {
	s, root := fixture(t)
	s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-unrelated", Value: []byte("202"), Sequence: 9})
	r1, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	s.Metadata[0], s.Metadata[1] = s.Metadata[1], s.Metadata[0]
	r2, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("restart/order changed receipt")
	}
	s.Metadata[1].OrganizationID = "org-B"
	r3, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Digest == r1.Digest || r3.Accounts[0].OrganizationID != "org-B" {
		t.Fatal("receipt failed to bind owner")
	}
	s.Accounts[0].Deleted = 1
	r4, err := Preflight(context.Background(), s, root)
	if err != nil || r4.Digest == r3.Digest {
		t.Fatal("receipt failed to bind deletion")
	}
}

func TestPreflightMissingProfileNeverCreated(t *testing.T) {
	s, root := fixture(t)
	s.Accounts[0].ID = 8
	if _, err := Preflight(context.Background(), s, root); err == nil {
		t.Fatal("missing profile accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "101", "8")); !os.IsNotExist(err) {
		t.Fatal("profile created")
	}
}

func TestPreflightRejectsAliasAndCancellation(t *testing.T) {
	s, root := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Preflight(ctx, s, root); err == nil {
		t.Fatal("cancellation ignored")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "New-Item -ItemType Junction -Path $env:OWNERSHIP_TEST_ALIAS -Target $env:OWNERSHIP_TEST_ROOT | Out-Null")
		cmd.Env = append(os.Environ(), "OWNERSHIP_TEST_ALIAS="+alias, "OWNERSHIP_TEST_ROOT="+root)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v %s", err, output)
		}
	} else if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(alias) }()
	if _, err := Preflight(context.Background(), s, alias); err == nil {
		t.Fatal("alias accepted")
	}
}

func TestPreflightRejectsBindMountedSharedProfile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bind-mount identity coverage is Linux-specific")
	}
	s, root := fixture(t)
	second := filepath.Join(root, "101", "8")
	if err := os.MkdirAll(second, 0700); err != nil {
		t.Fatal(err)
	}
	mount := exec.Command("sudo", "-n", "mount", "--bind", filepath.Join(root, "101", "7"), second)
	if output, err := mount.CombinedOutput(); err != nil {
		t.Skipf("bind mount unavailable on this runner: %v %s", err, output)
	}
	defer func() {
		_ = exec.Command("sudo", "-n", "umount", second).Run()
	}()
	s.Accounts = append(s.Accounts, LegacyAccount{ID: 8, TenantID: 101, Platform: "1688", ProfileRef: "opaque-second-reference"})
	if _, err := Preflight(context.Background(), s, root); err == nil || !strings.Contains(err.Error(), "share one browser profile filesystem identity") {
		t.Fatalf("shared bind-mounted profile was not rejected: %v", err)
	}
}

func TestPreflightBoundsAndNonDirectory(t *testing.T) {
	s, root := fixture(t)
	tooLarge := s
	tooLarge.Metadata = make([]OrganizationMetadata, MaxRows+1)
	if _, err := Preflight(context.Background(), tooLarge, root); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
	if _, err := Preflight(context.Background(), s, "relative"); err == nil {
		t.Fatal("relative runtime root accepted")
	}
	s.Accounts[0].ID = 8
	if err := os.WriteFile(filepath.Join(root, "101", "8"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Preflight(context.Background(), s, root); err == nil {
		t.Fatal("file accepted as profile")
	}
}
