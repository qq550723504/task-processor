package ownershipmigration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestReceiptExclusiveCompletePublication(t *testing.T) {
	s, root := fixture(t)
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err = ValidateReceiptTarget(root, path, r); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- WriteReceipt(context.Background(), path, r) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("want one exclusive publisher, got %d", success)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Receipt
	if err = json.Unmarshal(data, &got); err != nil || got.Digest != r.Digest {
		t.Fatalf("partial/invalid receipt: %v", err)
	}
	if err = WriteReceipt(context.Background(), path, r); err == nil {
		t.Fatal("existing receipt overwritten")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}

func TestReceiptRejectsPartialOrTamperedEvidence(t *testing.T) {
	s, root := fixture(t)
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	r.Accounts[0].OrganizationID = "wrong-org"
	for _, r := range []Receipt{r, {}} {
		path := filepath.Join(t.TempDir(), "receipt.json")
		if err := WriteReceipt(context.Background(), path, r); err == nil {
			t.Fatal("invalid evidence published")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("failed validation left success file")
		}
	}
}

func TestReceiptTargetRejectsProfileTreeAndAlias(t *testing.T) {
	s, root := fixture(t)
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(t.TempDir(), "receipt.json")
	if err = ValidateReceiptTarget(root, valid, r); err != nil {
		t.Fatalf("valid outside target rejected: %v", err)
	}
	for _, path := range []string{
		filepath.Join(root, "receipt.json"),
		filepath.Join(root, "101", "7", "receipt.json"),
	} {
		if err = ValidateReceiptTarget(root, path, r); err == nil {
			t.Fatalf("profile-contained receipt accepted: %s", path)
		}
	}

	alias := filepath.Join(t.TempDir(), "profile-alias")
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "New-Item -ItemType Junction -Path $env:OWNERSHIP_TEST_ALIAS -Target $env:OWNERSHIP_TEST_ROOT | Out-Null")
		cmd.Env = append(os.Environ(), "OWNERSHIP_TEST_ALIAS="+alias, "OWNERSHIP_TEST_ROOT="+filepath.Join(root, "101", "7"))
		if output, createErr := cmd.CombinedOutput(); createErr != nil {
			t.Fatalf("create junction: %v %s", createErr, output)
		}
	} else if err = os.Symlink(filepath.Join(root, "101", "7"), alias); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(alias) }()
	if err = ValidateReceiptTarget(root, filepath.Join(alias, "receipt.json"), r); err == nil {
		t.Fatal("aliased profile receipt target accepted")
	}
}

func TestReceiptPublicationHonorsCancellationBeforeFinalLink(t *testing.T) {
	s, root := fixture(t)
	r, err := Preflight(context.Background(), s, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	ctx, cancel := context.WithCancel(context.Background())
	err = writeReceipt(ctx, path, r, cancel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled publication left final receipt: %v", statErr)
	}
	entries, readErr := os.ReadDir(filepath.Dir(path))
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("cancelled publication leaked staging files: %v %v", entries, readErr)
	}
}
