// Package ownershipmigration provides offline, read-only evidence for the one-time
// 1688 ownership migration. It must not be used by request or worker paths.
package ownershipmigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MaxRows = 100000

// LegacyAccount is migration input, not a business ownership contract.
type LegacyAccount struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"legacy_tenant_id"`
	Platform   string `json:"platform"`
	ProfileRef string `json:"profile_ref"`
	Status     int16  `json:"status"`
	Deleted    int16  `json:"deleted"`
}

type OrganizationMetadata struct {
	OrganizationID string `json:"organization_id"`
	Value          []byte `json:"value"`
	Sequence       int64  `json:"sequence"`
	OwnerRemoved   bool   `json:"owner_removed"`
}

type Observation struct {
	SourceID string    `json:"source_id"`
	Database string    `json:"database"`
	At       time.Time `json:"observed_at"`
}

type Snapshot struct {
	Accounts            []LegacyAccount
	Metadata            []OrganizationMetadata
	AccountObservation  Observation
	MetadataObservation Observation
}

type AccountEvidence struct {
	OrganizationID   string        `json:"organization_id"`
	ProfileDirectory string        `json:"profile_directory"`
	Previous         LegacyAccount `json:"previous"`
}

// Receipt is preview evidence only. It does not authorize a backfill or certify
// projection freshness, account access, all runtime volumes or drained old jobs.
type Receipt struct {
	Version             int                    `json:"version"`
	Stage               string                 `json:"stage"`
	SnapshotConsistency string                 `json:"snapshot_consistency"`
	AccountObservation  Observation            `json:"account_observation"`
	MetadataObservation Observation            `json:"metadata_observation"`
	Accounts            []AccountEvidence      `json:"accounts"`
	Metadata            []OrganizationMetadata `json:"metadata"`
	Digest              string                 `json:"sha256"`
}

type profileDirectoryIdentity struct {
	accountID int64
	info      os.FileInfo
}

func Preflight(ctx context.Context, s Snapshot, root string) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if len(s.Accounts) > MaxRows || len(s.Metadata) > MaxRows {
		return Receipt{}, fmt.Errorf("snapshot exceeds row limit")
	}
	if !filepath.IsAbs(root) {
		return Receipt{}, fmt.Errorf("absolute runtime profile root required")
	}
	root = filepath.Clean(root)
	if _, err := verifyDirectoryInfo(root); err != nil {
		return Receipt{}, err
	}
	metadata := append([]OrganizationMetadata(nil), s.Metadata...)
	sort.Slice(metadata, func(i, j int) bool { return metadata[i].OrganizationID < metadata[j].OrganizationID })
	owners := map[int64]string{}
	seenOrgs := map[string]bool{}
	for _, m := range metadata {
		if err := ctx.Err(); err != nil {
			return Receipt{}, err
		}
		if m.OrganizationID == "" || strings.TrimSpace(m.OrganizationID) != m.OrganizationID || seenOrgs[m.OrganizationID] {
			return Receipt{}, fmt.Errorf("invalid or ambiguous Organization metadata")
		}
		seenOrgs[m.OrganizationID] = true
		if m.OwnerRemoved {
			continue
		}
		v, err := strconv.ParseInt(string(m.Value), 10, 64)
		if err != nil || v <= 0 || strconv.FormatInt(v, 10) != string(m.Value) {
			return Receipt{}, fmt.Errorf("invalid Organization mapping value")
		}
		if _, exists := owners[v]; exists {
			return Receipt{}, fmt.Errorf("ambiguous Organization mapping")
		}
		owners[v] = m.OrganizationID
	}
	accounts := append([]LegacyAccount(nil), s.Accounts...)
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	r := Receipt{Version: 1, Stage: "preflight_only", SnapshotConsistency: "separate_non_atomic_snapshots", AccountObservation: s.AccountObservation, MetadataObservation: s.MetadataObservation, Metadata: metadata, Accounts: []AccountEvidence{}}
	var previousID int64
	var profileIdentities []profileDirectoryIdentity
	for _, a := range accounts {
		if err := ctx.Err(); err != nil {
			return Receipt{}, err
		}
		if a.ID <= 0 || a.ID == previousID || a.TenantID <= 0 || !strings.EqualFold(a.Platform, "1688") || strings.TrimSpace(a.ProfileRef) == "" {
			return Receipt{}, fmt.Errorf("invalid or duplicate source account %d", a.ID)
		}
		previousID = a.ID
		org, ok := owners[a.TenantID]
		if !ok {
			return Receipt{}, fmt.Errorf("missing Organization mapping for source account %d", a.ID)
		}
		dir := filepath.Join(root, strconv.FormatInt(a.TenantID, 10), strconv.FormatInt(a.ID, 10))
		info, err := verifyDirectoryInfo(dir)
		if err != nil {
			return Receipt{}, fmt.Errorf("source account %d: %w", a.ID, err)
		}
		for _, existing := range profileIdentities {
			if os.SameFile(info, existing.info) {
				return Receipt{}, fmt.Errorf("source accounts %d and %d share one browser profile filesystem identity", existing.accountID, a.ID)
			}
		}
		profileIdentities = append(profileIdentities, profileDirectoryIdentity{accountID: a.ID, info: info})
		r.Accounts = append(r.Accounts, AccountEvidence{OrganizationID: org, ProfileDirectory: dir, Previous: a})
	}
	// Observation times vary on a fresh read; bind source identities and all
	// captured migration inputs while permitting same-input restart comparison.
	r.Digest = receiptDigest(r)
	return r, nil
}

func receiptDigest(r Receipt) string {
	canonical := r
	canonical.Digest = ""
	canonical.AccountObservation.At = time.Time{}
	canonical.MetadataObservation.At = time.Time{}
	payload, _ := json.Marshal(canonical) // only concrete JSON-compatible fields
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func verifyDirectory(path string) error {
	_, err := verifyDirectoryInfo(path)
	return err
}

func verifyDirectoryInfo(path string) (os.FileInfo, error) {
	// Lstat every ancestor as well as EvalSymlinks: a junction/symlink must not
	// silently bind the receipt to a different profile or volume.
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("profile directory missing, non-directory or aliased")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(path) {
		return nil, fmt.Errorf("profile directory resolution changed")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("profile directory missing or non-directory")
	}
	return info, nil
}
