//go:build linux

package ownershipmigration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
