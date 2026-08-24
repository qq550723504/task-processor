package identitypreflight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/listingkit/userdirectory"
)

type Service struct {
	owners               OwnerRepository
	directory            userdirectory.Directory
	legacyTenantResolver LegacyTenantOrganizationResolver
	output               io.Writer
}

type LegacyTenantOrganizationResolver interface {
	ResolveOrganizationID(ctx context.Context, legacyTenantID int64) (string, bool, error)
}

func NewService(
	owners OwnerRepository,
	directory userdirectory.Directory,
	legacyTenantResolver LegacyTenantOrganizationResolver,
	output io.Writer,
) *Service {
	if output == nil {
		output = io.Discard
	}
	return &Service{
		owners:               owners,
		directory:            directory,
		legacyTenantResolver: legacyTenantResolver,
		output:               output,
	}
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
	Reason   string
}

type normalizedOwner struct {
	PersistedOwner
	OrganizationID string
}

func (service *Service) Run(ctx context.Context) error {
	owners, err := service.owners.List(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("identity preflight: %w", ctxErr)
		}
		return errors.New("identity preflight: list persisted owners failed")
	}
	missingSubjectFindings := make([]unknownOwnerFinding, 0)
	for _, owner := range owners {
		if strings.TrimSpace(owner.UserID) == "" {
			missingSubjectFindings = append(missingSubjectFindings, unknownOwnerFinding{
				Table:    owner.Table,
				TenantID: owner.TenantID,
				UserID:   owner.UserID,
				RowCount: owner.RowCount,
				Reason:   "missing_subject",
			})
		}
	}
	if len(missingSubjectFindings) > 0 {
		return service.reportFindings(missingSubjectFindings)
	}

	normalizedOwners, err := service.normalizeOwnerTenants(ctx, owners)
	if err != nil {
		return err
	}

	ownersByTenant := make(map[string][]normalizedOwner)
	for _, owner := range normalizedOwners {
		ownersByTenant[owner.OrganizationID] = append(ownersByTenant[owner.OrganizationID], owner)
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
	for _, owner := range normalizedOwners {
		key := identityKey{TenantID: owner.OrganizationID, Subject: owner.UserID}
		if _, found := knownIdentities[key]; found {
			continue
		}
		findings = append(findings, unknownOwnerFinding{
			Table:    owner.Table,
			TenantID: owner.TenantID,
			UserID:   owner.UserID,
			RowCount: owner.RowCount,
			Reason:   "unknown_subject",
		})
	}
	return service.reportFindings(findings)
}

func (service *Service) reportFindings(findings []unknownOwnerFinding) error {
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
		if left.Reason != right.Reason {
			return left.Reason < right.Reason
		}
		return left.RowCount < right.RowCount
	})

	for _, finding := range findings {
		if _, err := fmt.Fprintf(
			service.output,
			"status=blocked table=%s tenant=%s owner=%s rows=%d reason=%s\n",
			finding.Table,
			fingerprint(finding.TenantID),
			fingerprint(finding.UserID),
			finding.RowCount,
			finding.Reason,
		); err != nil {
			return errors.New("identity preflight: write report failed")
		}
	}
	if len(findings) > 0 {
		return &ErrUnknownOwners{Count: len(findings)}
	}
	return nil
}

func (service *Service) normalizeOwnerTenants(ctx context.Context, owners []PersistedOwner) ([]normalizedOwner, error) {
	legacyOrganizations := make(map[int64]string)
	normalized := make([]normalizedOwner, 0, len(owners))
	for _, owner := range owners {
		organizationID := owner.TenantID
		if owner.TenantDomain == TenantDomainLegacyNumeric {
			legacyTenantID := strings.TrimSpace(owner.TenantID)
			parsed, parseErr := strconv.ParseInt(legacyTenantID, 10, 64)
			if parseErr != nil || parsed <= 0 || service.legacyTenantResolver == nil {
				return nil, errors.New("identity preflight: resolve tenant organization failed")
			}
			if cached, ok := legacyOrganizations[parsed]; ok {
				organizationID = cached
			} else {
				resolved, found, resolveErr := service.legacyTenantResolver.ResolveOrganizationID(ctx, parsed)
				if resolveErr != nil {
					if ctxErr := ctx.Err(); ctxErr != nil {
						return nil, fmt.Errorf("identity preflight: %w", ctxErr)
					}
					return nil, errors.New("identity preflight: resolve tenant organization failed")
				}
				organizationID = strings.TrimSpace(resolved)
				if !found || organizationID == "" {
					return nil, errors.New("identity preflight: resolve tenant organization failed")
				}
				legacyOrganizations[parsed] = organizationID
			}
		}
		normalized = append(normalized, normalizedOwner{PersistedOwner: owner, OrganizationID: organizationID})
	}
	return normalized, nil
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return "sha256:" + hex.EncodeToString(sum[:6])
}
