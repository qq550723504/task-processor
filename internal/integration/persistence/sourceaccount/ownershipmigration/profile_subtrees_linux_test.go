//go:build linux

package ownershipmigration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestPreflightRejectsLinuxDescendantSymlinksWithoutPublicationOrMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		linkAt func(first, second string) string
		target func(first, second, link string) string
	}{
		{
			name:   "cross-account relative",
			linkAt: func(_, second string) string { return filepath.Join(second, "Default") },
			target: func(first, _, link string) string {
				target := filepath.Join(first, "Default")
				relative, err := filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
				return relative
			},
		},
		{
			name:   "cross-account absolute",
			linkAt: func(_, second string) string { return filepath.Join(second, "Default") },
			target: func(first, _, _ string) string { return filepath.Join(first, "Default") },
		},
		{
			name:   "deep relative",
			linkAt: func(_, second string) string { return filepath.Join(second, "Default", "Cache", "alias") },
			target: func(first, _, link string) string {
				target := filepath.Join(first, "Default", "Cache")
				relative, err := filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
				return relative
			},
		},
		{
			name:   "deep absolute",
			linkAt: func(_, second string) string { return filepath.Join(second, "Default", "Cache", "alias") },
			target: func(first, _, _ string) string { return filepath.Join(first, "Default", "Cache") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := twoAccountLinuxFixture(t)
			first := filepath.Join(root, "101", "7")
			second := filepath.Join(root, "202", "8")
			link := tc.linkAt(first, second)
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			target := tc.target(first, second, link)
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			receiptPath := filepath.Join(supportedTestTempDir(t), "receipt.json")
			r, err := Preflight(context.Background(), s, root)
			if err == nil {
				if targetErr := ValidateReceiptTarget(root, receiptPath, r); targetErr != nil {
					t.Fatalf("unsafe evidence reached receipt validation: %v", targetErr)
				}
				if writeErr := WriteReceipt(context.Background(), receiptPath, r); writeErr != nil {
					t.Fatalf("unsafe evidence reached receipt publication: %v", writeErr)
				}
				t.Fatal("descendant symlink produced and published usable evidence")
			}
			if r.Digest != "" {
				t.Fatal("rejected preflight returned a usable digest")
			}
			if _, err := os.Lstat(receiptPath); !os.IsNotExist(err) {
				t.Fatalf("rejected preflight published a final receipt: %v", err)
			}
			after, err := os.Lstat(link)
			if err != nil || after.Mode() != before.Mode() {
				t.Fatalf("validation modified the link: before=%v after=%v err=%v", before.Mode(), after, err)
			}
			if got, err := os.Readlink(link); err != nil || got != target {
				t.Fatalf("validation changed the link target: got=%q err=%v", got, err)
			}
			assertLinuxFixtureContent(t, first)
		})
	}
}

func TestPreflightRejectsBrokenAndLoopingLinuxDescendantSymlinks(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{name: "broken", target: "missing-target"},
		{name: "self-loop", target: "alias"},
		{name: "two-link-loop", target: "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := fixture(t)
			profile := filepath.Join(root, "101", "7")
			link := filepath.Join(profile, "alias")
			if err := os.Symlink(tc.target, link); err != nil {
				t.Fatal(err)
			}
			if tc.name == "two-link-loop" {
				if err := os.Symlink("alias", filepath.Join(profile, "other")); err != nil {
					t.Fatal(err)
				}
			}
			started := time.Now()
			r, err := Preflight(context.Background(), s, root)
			if err == nil || r.Digest != "" {
				t.Fatal("broken or looping descendant symlink produced usable evidence")
			}
			if time.Since(started) > time.Second {
				t.Fatal("validation appears to have followed a symlink loop")
			}
			if _, err := os.Lstat(link); err != nil {
				t.Fatalf("validation modified the symlink: %v", err)
			}
		})
	}
}

func TestLinuxProfileEntryWalkAcceptsOrdinaryEntries(t *testing.T) {
	s, root := fixture(t)
	profile := filepath.Join(root, "101", "7")
	if err := os.MkdirAll(filepath.Join(profile, "Default", "Cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "Default", "Preferences"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Preflight(context.Background(), s, root)
	if err != nil || r.Digest == "" {
		t.Fatalf("ordinary profile entries rejected: %v", err)
	}
}

func TestPreflightRejectsCrossAccountLinuxHardLinks(t *testing.T) {
	s, root := twoAccountLinuxFixture(t)
	first := filepath.Join(root, "101", "7", "Default", "Cookies")
	second := filepath.Join(root, "202", "8", "Default", "Cookies")
	if err := os.MkdirAll(filepath.Dir(second), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("shared fixture state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	r, err := Preflight(context.Background(), s, root)
	if err == nil || r.Digest != "" {
		t.Fatal("cross-account hard-linked profile state produced usable evidence")
	}
	left, leftErr := os.Stat(first)
	right, rightErr := os.Stat(second)
	if leftErr != nil || rightErr != nil || !os.SameFile(left, right) {
		t.Fatalf("validation modified hard links: %v %v", leftErr, rightErr)
	}
}

func TestPreflightAcceptsSameAccountLinuxHardLinks(t *testing.T) {
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

type cancelAfterChecksContext struct {
	context.Context
	checks   atomic.Int64
	cancelAt int64
}

func (c *cancelAfterChecksContext) Err() error {
	if c.checks.Add(1) >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestLinuxProfileEntryWalkHonorsCancellationDuringTraversal(t *testing.T) {
	profile := supportedTestTempDir(t)
	for i := 0; i < 20; i++ {
		if err := os.Mkdir(filepath.Join(profile, fmt.Sprintf("entry-%02d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: 7}
	err := validateLinuxProfileEntries(ctx, []AccountEvidence{{ProfileDirectory: profile, Previous: LegacyAccount{ID: 7}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want traversal cancellation, got %v", err)
	}
}

func TestLinuxProfileEntryWalkFailsClosedOnReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("run as an unprivileged user to verify directory read failure")
	}
	profile := supportedTestTempDir(t)
	unreadable := filepath.Join(profile, "unreadable")
	if err := os.Mkdir(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0700) })
	if err := validateLinuxProfileEntries(context.Background(), []AccountEvidence{{ProfileDirectory: profile, Previous: LegacyAccount{ID: 7}}}); err == nil {
		t.Fatal("unreadable descendant produced successful validation")
	}
}

func TestLinuxProfileEntryWalkResourceBoundary(t *testing.T) {
	profile := supportedTestTempDir(t)
	const entries = 4096
	for i := 0; i < entries; i++ {
		if err := os.WriteFile(filepath.Join(profile, fmt.Sprintf("entry-%04d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: entries + 100}
	started := time.Now()
	if err := validateLinuxProfileEntries(ctx, []AccountEvidence{{ProfileDirectory: profile, Previous: LegacyAccount{ID: 7}}}); err != nil {
		t.Fatal(err)
	}
	if got := ctx.checks.Load(); got < entries || got > entries+2 {
		t.Fatalf("entry checks=%d, want one bounded check per directory entry", got)
	}
	t.Logf("walked %d real tmpfs entries in %s", entries, time.Since(started))
}

func twoAccountLinuxFixture(t *testing.T) (Snapshot, string) {
	t.Helper()
	s, root := fixture(t)
	for _, dir := range []string{
		filepath.Join(root, "101", "7", "Default", "Cache"),
		filepath.Join(root, "202", "8"),
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	s.Accounts = append(s.Accounts, LegacyAccount{ID: 8, TenantID: 202, Platform: "1688", ProfileRef: "second"})
	s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-B", Value: []byte("202")})
	return s, root
}

func assertLinuxFixtureContent(t *testing.T, first string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(first, "sentinel"))
	if err != nil || string(data) != "existing session" {
		t.Fatalf("validation modified fixture content: %q %v", data, err)
	}
}
