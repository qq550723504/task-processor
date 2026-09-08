package sourceaccountownershiprehearsal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/integration/persistence/sourceaccount/ownershipmigration"
)

type rehearsalStorage struct {
	Root        string
	ProfileRoot string
	EvidenceDir string
	ReceiptPath string
}

func newRehearsalStorage(ctx context.Context) (rehearsalStorage, error) {
	for _, candidate := range rehearsalStorageCandidates() {
		if err := ctx.Err(); err != nil {
			return rehearsalStorage{}, err
		}
		if candidate == "" {
			continue
		}
		root, err := os.MkdirTemp(candidate, "source-account-b2-")
		if err != nil {
			continue
		}
		storage := rehearsalStorage{
			Root:        root,
			ProfileRoot: filepath.Join(root, "profiles"),
			EvidenceDir: filepath.Join(root, "evidence"),
		}
		storage.ReceiptPath = filepath.Join(storage.EvidenceDir, "preflight.json")
		if err = prepareSyntheticProfiles(storage); err == nil {
			err = validateSyntheticStorage(ctx, storage)
		}
		if err == nil {
			return storage, nil
		}
		_ = os.RemoveAll(root)
	}
	return rehearsalStorage{}, errors.New("no A-supported local rehearsal storage is available")
}

func prepareSyntheticProfiles(storage rehearsalStorage) error {
	accountDirectories := []string{
		filepath.Join(storage.ProfileRoot, "101", "7"),
		filepath.Join(storage.ProfileRoot, "102", "8"),
		filepath.Join(storage.ProfileRoot, "103", "9"),
	}
	for _, directory := range append(accountDirectories, storage.EvidenceDir) {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create synthetic rehearsal directory: %w", err)
		}
	}
	for _, directory := range accountDirectories {
		if err := os.WriteFile(filepath.Join(directory, "profile.marker"), []byte("synthetic-b2-profile\n"), 0o600); err != nil {
			return fmt.Errorf("create synthetic profile marker: %w", err)
		}
	}
	return nil
}

func validateSyntheticStorage(ctx context.Context, storage rehearsalStorage) error {
	now := time.Unix(1_725_734_400, 0).UTC()
	snapshot := ownershipmigration.Snapshot{
		Accounts: []ownershipmigration.LegacyAccount{
			{ID: 7, TenantID: 101, Platform: "1688", ProfileRef: "profile-7", Status: 1},
			{ID: 8, TenantID: 102, Platform: "1688", ProfileRef: "profile-8", Status: 0},
			{ID: 9, TenantID: 103, Platform: "1688", ProfileRef: "profile-9", Status: 1, Deleted: 1},
		},
		Metadata: []ownershipmigration.OrganizationMetadata{
			{OrganizationID: "org-b2-enabled", Value: []byte("101"), Sequence: 1},
			{OrganizationID: "org-b2-disabled", Value: []byte("102"), Sequence: 2},
			{OrganizationID: "org-b2-deleted", Value: []byte("103"), Sequence: 3},
		},
		AccountObservation:  ownershipmigration.Observation{SourceID: "issue364/storage/source", Database: "source_b2", At: now},
		MetadataObservation: ownershipmigration.Observation{SourceID: "issue364/storage/metadata", Database: "metadata_b2", At: now},
	}
	receipt, err := ownershipmigration.Preflight(ctx, snapshot, storage.ProfileRoot)
	if err != nil {
		return err
	}
	return ownershipmigration.ValidateReceiptTarget(storage.ProfileRoot, storage.ReceiptPath, receipt)
}

func (storage rehearsalStorage) cleanup() error {
	if storage.Root == "" || !filepath.IsAbs(storage.Root) {
		return errors.New("invalid rehearsal cleanup root")
	}
	clean := filepath.Clean(storage.Root)
	allowed := false
	for _, candidate := range rehearsalStorageCandidates() {
		if candidate == "" {
			continue
		}
		base, err := filepath.Abs(candidate)
		if err == nil {
			relative, relErr := filepath.Rel(filepath.Clean(base), clean)
			if relErr == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) &&
				!strings.HasPrefix(relative, ".."+string(os.PathSeparator)) &&
				strings.HasPrefix(filepath.Base(clean), "source-account-b2-") {
				allowed = true
				break
			}
		}
	}
	if !allowed || filepath.Base(clean) == "." {
		return errors.New("rehearsal cleanup root is outside fixed candidate storage")
	}
	return os.RemoveAll(clean)
}
