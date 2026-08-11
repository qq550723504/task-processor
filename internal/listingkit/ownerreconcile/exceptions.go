package ownerreconcile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SystemOwnedException is an audited, exact exception for one persisted
// owner-reconciliation candidate group. Fingerprints intentionally avoid
// carrying raw tenant or legacy-user identifiers outside the database scan.
type SystemOwnedException struct {
	Table                string
	TenantFingerprint    string
	CandidateFingerprint string
	ReportFingerprint    string
	Reason               string
}

// ExceptionStore supplies the active, explicitly approved system-owned keys.
type ExceptionStore interface {
	ListActive(context.Context) ([]SystemOwnedException, error)
}

const systemOwnedExceptionQuery = `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason
FROM listingkit_owner_scope_system_owned_exceptions
WHERE active = TRUE
ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`

type postgresExceptionStore struct {
	db *sql.DB
}

// NewPostgresExceptionStore constructs the read-only exception registry store.
func NewPostgresExceptionStore(db *sql.DB) ExceptionStore {
	return postgresExceptionStore{db: db}
}

func (store postgresExceptionStore) ListActive(ctx context.Context) ([]SystemOwnedException, error) {
	if store.db == nil {
		return nil, errors.New("owner exception registry is unavailable")
	}
	rows, err := store.db.QueryContext(ctx, systemOwnedExceptionQuery)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("query owner exception registry failed")
	}
	defer rows.Close()
	result := make([]SystemOwnedException, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var item SystemOwnedException
		if err := rows.Scan(&item.Table, &item.TenantFingerprint, &item.CandidateFingerprint, &item.ReportFingerprint, &item.Reason); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, errors.New("scan owner exception registry failed")
		}
		if err := validateSystemOwnedException(item); err != nil {
			return nil, err
		}
		key := systemOwnedExceptionKey(item.Table, item.TenantFingerprint, item.CandidateFingerprint)
		if _, exists := seen[key]; exists {
			return nil, errors.New("owner exception registry contains duplicate keys")
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("iterate owner exception registry failed")
	}
	return result, nil
}

var fingerprintPattern = regexp.MustCompile(`^sha256:[0-9a-f]{12}$`)
var reportFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

func validateSystemOwnedException(item SystemOwnedException) error {
	if !ownerReconcileIdentifier.MatchString(strings.TrimSpace(item.Table)) ||
		!fingerprintPattern.MatchString(strings.TrimSpace(item.TenantFingerprint)) ||
		!fingerprintPattern.MatchString(strings.TrimSpace(item.CandidateFingerprint)) ||
		!reportFingerprintPattern.MatchString(strings.TrimSpace(item.ReportFingerprint)) ||
		strings.TrimSpace(item.Reason) == "" {
		return errors.New("owner exception registry contains an invalid exception")
	}
	return nil
}

func systemOwnedExceptionKey(table, tenantFingerprint, candidateFingerprint string) string {
	return fmt.Sprintf("%s|%s|%s", table, tenantFingerprint, candidateFingerprint)
}
