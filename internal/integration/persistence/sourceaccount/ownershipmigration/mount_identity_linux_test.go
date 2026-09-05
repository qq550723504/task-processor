//go:build linux

package ownershipmigration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
