package ownerreconcile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	ApprovedSystemOwnedExceptionReport = "648cdfab03c4"
	ApprovedSystemOwnedExceptionGroups = 312
	ApprovedSystemOwnedExceptionRows   = int64(874891)
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
	Rows                 int64
}

// ExceptionStore supplies the active, explicitly approved system-owned keys.
type ExceptionStore interface {
	ListActive(context.Context) ([]SystemOwnedException, error)
}

const systemOwnedExceptionQuery = `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason, row_count
FROM listingkit_owner_scope_system_owned_exceptions
WHERE active = TRUE
ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`

type postgresExceptionStore struct {
	queryer Queryer
}

// NewPostgresExceptionStore constructs the read-only exception registry store.
func NewPostgresExceptionStore(db *sql.DB) ExceptionStore {
	return postgresExceptionStore{queryer: db}
}

func newPostgresExceptionStore(queryer Queryer) ExceptionStore {
	return postgresExceptionStore{queryer: queryer}
}

func (store postgresExceptionStore) ListActive(ctx context.Context) ([]SystemOwnedException, error) {
	if store.queryer == nil {
		return nil, errors.New("owner exception registry is unavailable")
	}
	rows, err := store.queryer.QueryContext(ctx, systemOwnedExceptionQuery)
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
		if err := rows.Scan(&item.Table, &item.TenantFingerprint, &item.CandidateFingerprint, &item.ReportFingerprint, &item.Reason, &item.Rows); err != nil {
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
		strings.TrimSpace(item.ReportFingerprint) != ApprovedSystemOwnedExceptionReport ||
		item.Rows <= 0 ||
		strings.TrimSpace(item.Reason) == "" {
		return errors.New("owner exception registry contains an invalid exception")
	}
	return nil
}

func systemOwnedExceptionKey(table, tenantFingerprint, candidateFingerprint string) string {
	return fmt.Sprintf("%s|%s|%s", table, tenantFingerprint, candidateFingerprint)
}

// ValidateApprovedExceptionReport accepts only the reviewed production
// snapshot. The fixed fingerprint and counts prevent accidentally seeding a
// later or broader report through the one-shot operator command.
func ValidateApprovedExceptionReport(report Report, confirmation string) error {
	if strings.TrimSpace(confirmation) != ApprovedSystemOwnedExceptionReport ||
		strings.TrimSpace(report.ReportFingerprint) != ApprovedSystemOwnedExceptionReport {
		return errors.New("approved owner exception report confirmation mismatch")
	}
	if len(report.Findings) != ApprovedSystemOwnedExceptionGroups ||
		report.Summary.FindingGroups != ApprovedSystemOwnedExceptionGroups ||
		report.Summary.UnresolvedRows != ApprovedSystemOwnedExceptionRows {
		return errors.New("approved owner exception report shape changed")
	}
	for _, finding := range report.Findings {
		if finding.Reason != "unmapped_candidate" {
			return errors.New("approved owner exception report contains a non-unmapped finding")
		}
		if !fingerprintPattern.MatchString(finding.TenantFingerprint) || !fingerprintPattern.MatchString(finding.OwnerFingerprint) || !ownerReconcileIdentifier.MatchString(finding.Table) {
			return errors.New("approved owner exception report contains an invalid finding")
		}
	}
	fingerprint, err := report.Fingerprint()
	if err != nil || fingerprint != ApprovedSystemOwnedExceptionReport {
		return errors.New("approved owner exception report fingerprint changed")
	}
	return nil
}

// InsertSystemOwnedExceptions inserts the approved exception set without
// touching any business table. Re-running the command is idempotent.
func InsertSystemOwnedExceptions(ctx context.Context, db *sql.DB, report Report, reason string) (int, error) {
	if db == nil {
		return 0, errors.New("owner exception registry is unavailable")
	}
	if strings.TrimSpace(reason) == "" {
		return 0, errors.New("owner exception reason is required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		return 0, errors.New("begin owner exception registry transaction failed")
	}
	inserted := 0
	const insertQuery = `INSERT INTO listingkit_owner_scope_system_owned_exceptions
    (table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason, row_count, active)
VALUES ($1, $2, $3, $4, $5, $6, TRUE)
ON CONFLICT (table_name, tenant_fingerprint, candidate_fingerprint) DO UPDATE SET
    report_fingerprint = EXCLUDED.report_fingerprint,
    reason = EXCLUDED.reason,
    row_count = EXCLUDED.row_count`
	for _, finding := range report.Findings {
		result, execErr := tx.ExecContext(ctx, insertQuery, finding.Table, finding.TenantFingerprint, finding.OwnerFingerprint, report.ReportFingerprint, reason, finding.Rows)
		if execErr != nil {
			_ = tx.Rollback()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return 0, ctxErr
			}
			return 0, errors.New("insert owner exception registry failed")
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr == nil && affected > 0 {
			inserted += int(affected)
		}
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return 0, errors.New("commit owner exception registry failed")
	}
	return inserted, nil
}
