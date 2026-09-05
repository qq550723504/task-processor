//go:build linux

package ownershipmigration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileSubtreeMountMatrix(t *testing.T) {
	base := "36 25 0:42 / / rw - ext4 /dev/root rw\n"
	accounts := []AccountEvidence{
		{ProfileDirectory: "/profiles/101/7", Previous: LegacyAccount{ID: 7}},
		{ProfileDirectory: "/profiles/202/8", Previous: LegacyAccount{ID: 8}},
	}
	for _, tc := range []struct {
		name, extra string
		invalid     bool
	}{
		{"distinct profiles", "", false},
		{"descendant account bind", "50 36 0:42 /profiles/101/7/Default /profiles/202/8 rw - ext4 /dev/root rw\n", true},
		{"reverse descendant account bind", "50 36 0:42 /profiles/202/8/Default /profiles/101/7 rw - ext4 /dev/root rw\n", true},
		{"nested sharing across devices", "50 36 0:99 / /profiles/101/7/Default rw - tmpfs tmpfs rw\n51 36 0:99 / /profiles/202/8/Default rw - tmpfs tmpfs rw\n", true},
		{"same account overlap", "50 36 0:42 /profiles/101/7 /profiles/101/7/Default rw - ext4 /dev/root rw\n", false},
		{"separate devices", "50 36 0:99 / /profiles/101/7 rw - tmpfs tmpfs rw\n51 36 0:100 / /profiles/202/8 rw - tmpfs tmpfs rw\n", false},
		{"similar backing prefix", "50 36 0:99 /a /profiles/101/7 rw - tmpfs tmpfs rw\n51 36 0:99 /a-sibling /profiles/202/8 rw - tmpfs tmpfs rw\n", false},
		{"unrelated opaque root", "50 36 0:99 net:[1234] /run/netns/test rw - nsfs nsfs rw\n", false},
		{"unknown protected root", "50 36 0:99 unknown /profiles/101/7/Default rw - nsfs nsfs rw\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateProfileSubtreesFromMountInfo(context.Background(), accounts, []byte(base+tc.extra)); (err != nil) != tc.invalid {
				t.Fatalf("invalid=%v, error=%v", tc.invalid, err)
			}
		})
	}
}

func TestProfileSubtreeIndexAtInventoryLimit(t *testing.T) {
	accounts := make([]AccountEvidence, MaxRows)
	var mounts strings.Builder
	mounts.WriteString("36 25 0:42 / / rw - ext4 /dev/root rw\n")
	for i := range accounts {
		dir := fmt.Sprintf("/profiles/101/%d", i+1)
		accounts[i] = AccountEvidence{ProfileDirectory: dir, Previous: LegacyAccount{ID: int64(i + 1)}}
		fmt.Fprintf(&mounts, "%d 36 0:99 /private/%d %s/Default rw - tmpfs tmpfs rw\n", i+50, i+1, dir)
	}
	if err := validateProfileSubtreesFromMountInfo(context.Background(), accounts, []byte(mounts.String())); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateProfileSubtreesFromMountInfo(ctx, accounts, []byte(mounts.String())); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestReceiptMountMatrix(t *testing.T) {
	base := "36 25 0:42 / / rw - ext4 /dev/root rw\n"
	nested := "50 36 0:99 / /tmp/profiles/101/7/Default rw - tmpfs tmpfs rw\n"
	for _, tc := range []struct {
		name, parent, mounts string
		alias, invalid       bool
	}{
		{"root", "/tmp/profiles", base, true, false},
		{"account", "/tmp/profiles/101/7", base, true, false},
		{"descendant", "/tmp/profiles/101/7/Default", base, true, false},
		{"external", "/tmp/evidence", base, false, false},
		{"similar prefix", "/tmp/profiles-other", base, false, false},
		{"account bind", "/tmp/evidence", base + "51 36 0:42 /tmp/profiles/101/7 /tmp/evidence rw - ext4 /dev/root rw\n", true, false},
		{"descendant bind", "/tmp/evidence/child", base + "51 36 0:42 /tmp/profiles/101/7/Default /tmp/evidence rw - ext4 /dev/root rw\n", true, false},
		{"nested device bind", "/tmp/evidence", base + nested + "51 36 0:99 / /tmp/evidence rw - tmpfs tmpfs rw\n", true, false},
		{"nested device subtree bind", "/tmp/evidence/child", base + nested + "51 36 0:99 /Cache /tmp/evidence rw - tmpfs tmpfs rw\n", true, false},
		{"unrelated device", "/tmp/evidence", base + nested + "51 36 0:100 / /tmp/evidence rw - tmpfs tmpfs rw\n", false, false},
		{"empty mountinfo", "/tmp/evidence", "", false, true},
		{"malformed mountinfo", "/tmp/evidence", base + "invalid\n", false, true},
		{"uncovered path", "/tmp/evidence", nested, false, true},
		{"ambiguous mounts", "/tmp/evidence", base + "51 36 0:99 / / rw - tmpfs tmpfs rw\n", false, true},
		{"relative mount root", "/tmp/evidence", base + "51 36 0:99 invalid /tmp/evidence rw - tmpfs tmpfs rw\n", false, true},
		{"unrelated stacked mounts", "/tmp/evidence", base + "51 36 0:99 / /unrelated rw - tmpfs tmpfs rw\n52 51 0:100 / /unrelated rw - tmpfs tmpfs rw\n", false, false},
		{"unrelated namespace file", "/tmp/evidence", base + "51 36 0:99 net:[1234] /run/netns/test rw - nsfs nsfs rw\n", false, false},
		{"identical backing locations", "/tmp/evidence", base + "51 36 0:42 / / rw - ext4 /dev/root rw\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alias, err := receiptParentAliasesProfileSubtreeFromMountInfo("/tmp/profiles", tc.parent, []byte(tc.mounts))
			if (err != nil) != tc.invalid || alias != tc.alias {
				t.Fatalf("alias=%v err=%v; want alias=%v invalid=%v", alias, err, tc.alias, tc.invalid)
			}
		})
	}
}

func TestReceiptParentMountInfoRejectsProfileDescendantBindMount(t *testing.T) {
	mountInfo := []byte(
		"36 25 0:42 / / rw,relatime - ext4 /dev/root rw\n" +
			"50 36 0:42 /tmp/profiles/101/7/Default /tmp/evidence rw,relatime - ext4 /dev/root rw\n",
	)
	aliases, err := receiptParentAliasesProfileSubtreeFromMountInfo("/tmp/profiles", "/tmp/evidence", mountInfo)
	if err != nil {
		t.Fatal(err)
	}
	if !aliases {
		t.Fatal("bind-mounted browser profile descendant was not recognized")
	}
	aliases, err = receiptParentAliasesProfileSubtreeFromMountInfo("/tmp/profiles", "/tmp/ordinary-evidence", mountInfo)
	if err != nil {
		t.Fatal(err)
	}
	if aliases {
		t.Fatal("ordinary external directory was misclassified as a profile alias")
	}
}

func TestReceiptTargetRejectsLiveBindMountedProfileDescendant(t *testing.T) {
	s, root := fixture(t)
	profileDescendant := filepath.Join(root, "101", "7", "Default")
	if err := os.MkdirAll(profileDescendant, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	mountPoint := filepath.Join(t.TempDir(), "profile-descendant-bind")
	if err = os.Mkdir(mountPoint, 0700); err != nil {
		t.Fatal(err)
	}
	mount := exec.Command("sudo", "-n", "mount", "--bind", profileDescendant, mountPoint)
	if output, mountErr := mount.CombinedOutput(); mountErr != nil {
		t.Skipf("bind mount unavailable on this runner: %v %s", mountErr, output)
	}
	defer func() {
		_ = exec.Command("sudo", "-n", "umount", mountPoint).Run()
	}()
	if err = ValidateReceiptTarget(root, filepath.Join(mountPoint, "receipt.json"), r); err == nil {
		t.Fatal("receipt target backed by browser profile descendant was accepted")
	}
}

func TestReceiptTargetRejectsLiveNestedMount(t *testing.T) {
	s, root := fixture(t)
	descendant := filepath.Join(root, "101", "7", "Default")
	if err := os.Mkdir(descendant, 0700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("sudo", "-n", "mount", "-t", "tmpfs", "tmpfs", descendant).CombinedOutput(); err != nil {
		t.Skipf("nested mount unavailable: %v %s", err, output)
	}
	defer func() { _ = exec.Command("sudo", "-n", "umount", descendant).Run() }()
	outside := t.TempDir()
	if output, err := exec.Command("sudo", "-n", "mount", "--bind", descendant, outside).CombinedOutput(); err != nil {
		t.Skipf("external bind unavailable: %v %s", err, output)
	}
	defer func() { _ = exec.Command("sudo", "-n", "umount", outside).Run() }()
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateReceiptTarget(root, filepath.Join(outside, "receipt.json"), r); err == nil {
		t.Fatal("external bind of separately mounted profile descendant accepted")
	}
	entries, err := os.ReadDir(descendant)
	if err != nil || len(entries) != 0 {
		t.Fatalf("validation modified disposable profile descendant: %v %v", entries, err)
	}
}

func TestPreflightRejectsLiveOverlappingProfileSubtrees(t *testing.T) {
	for _, kind := range []string{"account bound to descendant", "descendant bound to descendant", "shared external subtree"} {
		t.Run(kind, func(t *testing.T) {
			s, root := fixture(t)
			first := filepath.Join(root, "101", "7")
			second := filepath.Join(root, "202", "8")
			for _, dir := range []string{filepath.Join(first, "Default"), filepath.Join(second, "Default")} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			bind := func(source, target string) {
				t.Helper()
				if output, err := exec.Command("sudo", "-n", "mount", "--bind", source, target).CombinedOutput(); err != nil {
					t.Skipf("bind mount unavailable: %v %s", err, output)
				}
				t.Cleanup(func() { _ = exec.Command("sudo", "-n", "umount", target).Run() })
			}
			switch kind {
			case "account bound to descendant":
				bind(filepath.Join(first, "Default"), second)
			case "descendant bound to descendant":
				bind(filepath.Join(first, "Default"), filepath.Join(second, "Default"))
			case "shared external subtree":
				shared := t.TempDir()
				bind(shared, filepath.Join(first, "Default"))
				bind(shared, filepath.Join(second, "Default"))
			}
			a, err := os.Stat(first)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.Stat(second)
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(a, b) {
				t.Fatal("fixture must have distinct account-root identities")
			}
			s.Accounts = append(s.Accounts, LegacyAccount{ID: 8, TenantID: 202, Platform: "1688", ProfileRef: "second"})
			s.Metadata = append(s.Metadata, OrganizationMetadata{OrganizationID: "org-B", Value: []byte("202")})
			if r, err := Preflight(context.Background(), s, root); err == nil || r.Digest != "" {
				t.Fatal("overlapping profile subtrees produced a usable receipt")
			}
		})
	}
}
