package identitypreflight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"task-processor/internal/listingkit/userdirectory"
)

type Service struct {
	owners    OwnerRepository
	directory userdirectory.Directory
	output    io.Writer
}

func NewService(owners OwnerRepository, directory userdirectory.Directory, output io.Writer) *Service {
	if output == nil {
		output = io.Discard
	}
	return &Service{owners: owners, directory: directory, output: output}
}

// ErrUnknownOwners reports the safe count of persisted owner mappings that do
// not exist in the matching tenant directory.
type ErrUnknownOwners struct {
	Count int
}

func (err *ErrUnknownOwners) Error() string {
	return fmt.Sprintf("identity preflight blocked: %d unknown owner mappings", err.Count)
}

type identityKey struct {
	TenantID string
	Subject  string
}

type unknownOwnerFinding struct {
	Table    string
	TenantID string
	UserID   string
	RowCount int64
}

func (service *Service) Run(ctx context.Context) error {
	owners, err := service.owners.List(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("identity preflight: %w", ctxErr)
		}
		return errors.New("identity preflight: list persisted owners failed")
	}

	ownersByTenant := make(map[string][]PersistedOwner)
	for _, owner := range owners {
		ownersByTenant[owner.TenantID] = append(ownersByTenant[owner.TenantID], owner)
	}
	tenants := make([]string, 0, len(ownersByTenant))
	for tenantID := range ownersByTenant {
		tenants = append(tenants, tenantID)
	}
	sort.Strings(tenants)

	knownIdentities := make(map[identityKey]struct{})
	for _, tenantID := range tenants {
		users, err := service.directory.ListByTenant(ctx, tenantID)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("identity preflight: %w", ctxErr)
			}
			return errors.New("identity preflight: load user directory failed")
		}
		for _, user := range users {
			knownIdentities[identityKey{TenantID: user.TenantID, Subject: user.Subject}] = struct{}{}
		}
	}

	findings := make([]unknownOwnerFinding, 0)
	for _, owner := range owners {
		key := identityKey{TenantID: owner.TenantID, Subject: owner.UserID}
		if _, found := knownIdentities[key]; found {
			continue
		}
		findings = append(findings, unknownOwnerFinding{
			Table:    owner.Table,
			TenantID: owner.TenantID,
			UserID:   owner.UserID,
			RowCount: owner.RowCount,
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.Table != right.Table {
			return left.Table < right.Table
		}
		if left.TenantID != right.TenantID {
			return left.TenantID < right.TenantID
		}
		if left.UserID != right.UserID {
			return left.UserID < right.UserID
		}
		return left.RowCount < right.RowCount
	})

	for _, finding := range findings {
		if _, err := fmt.Fprintf(
			service.output,
			"status=blocked table=%s tenant=%s owner=%s rows=%d reason=unknown_subject\n",
			finding.Table,
			fingerprint(finding.TenantID),
			fingerprint(finding.UserID),
			finding.RowCount,
		); err != nil {
			return errors.New("identity preflight: write report failed")
		}
	}
	if len(findings) > 0 {
		return &ErrUnknownOwners{Count: len(findings)}
	}
	return nil
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return "sha256:" + hex.EncodeToString(sum[:6])
}
