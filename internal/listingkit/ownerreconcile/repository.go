package ownerreconcile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type TenantDomain uint8

const (
	TenantDomainZITADELOrganization TenantDomain = iota
	TenantDomainLegacyNumeric
)

type CandidateColumn struct {
	Name   string
	Source string
}

type TableSpec struct {
	Table            string
	TenantDomain     TenantDomain
	Query            string
	UpdateQuery      string
	Columns          []string
	CandidateColumns []CandidateColumn
}

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type Repository struct {
	Queryer    Queryer
	Inventory  []TableSpec
	Identities []LegacyIdentity
	Beginner   interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}
}

type ApplySummary struct {
	RowsUpdated int64
	Batches     int
}

var ErrReportConfirmationMismatch = errors.New("owner reconciliation report confirmation mismatch")

var ownerReconcileIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (repository Repository) DryRun(ctx context.Context, identities []LegacyIdentity) (Report, error) {
	if repository.Queryer == nil {
		return Report{}, errors.New("owner reconciliation database is unavailable")
	}
	identityMap, err := indexLegacyIdentities(identities)
	if err != nil {
		return Report{}, err
	}
	findings := make([]Finding, 0)
	var autoRows int64
	for _, spec := range repository.Inventory {
		if err := validateTableSpec(spec); err != nil {
			return Report{}, err
		}
		rows, err := repository.Queryer.QueryContext(ctx, spec.Query)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Report{}, ctxErr
			}
			return Report{}, fmt.Errorf("query owner reconciliation table %s failed", spec.Table)
		}
		rowsFindings, rowsAuto, scanErr := scanTableRows(ctx, rows, spec, identityMap)
		if scanErr != nil {
			return Report{}, scanErr
		}
		findings = append(findings, rowsFindings...)
		autoRows += rowsAuto
	}
	return NewReport("", "", findings, autoRows), nil
}

// ApplyUnique revalidates the redacted report and applies only uniquely
// resolved groups through fixed, parameterized UPDATE statements. The caller
// must provide the exact report fingerprint it reviewed and the same value as
// the confirmation token.
func (repository Repository) ApplyUnique(ctx context.Context, reportFingerprint, expected string, batchSize int) (ApplySummary, error) {
	if strings.TrimSpace(reportFingerprint) == "" || strings.TrimSpace(reportFingerprint) != strings.TrimSpace(expected) {
		return ApplySummary{}, ErrReportConfirmationMismatch
	}
	if batchSize <= 0 {
		return ApplySummary{}, errors.New("batch size must be positive")
	}
	if repository.Queryer == nil || len(repository.Identities) == 0 {
		return ApplySummary{}, errors.New("owner reconciliation identities are unavailable")
	}
	current, err := repository.DryRun(ctx, repository.Identities)
	if err != nil {
		return ApplySummary{}, err
	}
	if err := current.SetFingerprint(); err != nil || current.ReportFingerprint != reportFingerprint {
		return ApplySummary{}, ErrReportConfirmationMismatch
	}
	beginner := repository.Beginner
	if beginner == nil {
		if db, ok := repository.Queryer.(*sql.DB); ok {
			beginner = db
		}
	}
	if beginner == nil {
		return ApplySummary{}, errors.New("owner reconciliation database cannot begin a write transaction")
	}
	identityMap, err := indexLegacyIdentities(repository.Identities)
	if err != nil {
		return ApplySummary{}, err
	}
	var summary ApplySummary
	for _, spec := range repository.Inventory {
		if strings.TrimSpace(spec.UpdateQuery) == "" || len(spec.CandidateColumns) == 0 {
			continue
		}
		groups, err := collectCandidateGroups(ctx, repository.Queryer, spec, identityMap)
		if err != nil {
			return ApplySummary{}, err
		}
		var tx *sql.Tx
		var txRows int64
		commit := func() error {
			if tx == nil {
				return nil
			}
			if err := tx.Commit(); err != nil {
				return errors.New("commit owner reconciliation transaction failed")
			}
			summary.Batches++
			tx = nil
			txRows = 0
			return nil
		}
		for _, group := range groups {
			if tx == nil {
				tx, err = beginner.BeginTx(ctx, nil)
				if err != nil {
					return ApplySummary{}, errors.New("begin owner reconciliation transaction failed")
				}
			}
			args := []any{group.Subject, group.TenantID}
			for _, value := range group.CandidateValues {
				args = append(args, value)
			}
			result, execErr := tx.ExecContext(ctx, spec.UpdateQuery, args...)
			if execErr != nil {
				_ = tx.Rollback()
				tx = nil
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ApplySummary{}, ctxErr
				}
				return ApplySummary{}, fmt.Errorf("apply owner reconciliation for %s failed", spec.Table)
			}
			updated, err := result.RowsAffected()
			if err != nil {
				_ = tx.Rollback()
				tx = nil
				return ApplySummary{}, errors.New("read owner reconciliation update count failed")
			}
			summary.RowsUpdated += updated
			txRows += updated
			if txRows >= int64(batchSize) {
				if err := commit(); err != nil {
					return ApplySummary{}, err
				}
			}
		}
		if err := commit(); err != nil {
			return ApplySummary{}, err
		}
	}
	return summary, nil
}

type candidateGroup struct {
	TenantID        string
	CandidateValues []string
	Subject         string
}

func collectCandidateGroups(ctx context.Context, queryer Queryer, spec TableSpec, identities map[string]string) (groups []candidateGroup, resultErr error) {
	if err := validateTableSpec(spec); err != nil {
		return nil, err
	}
	rows, err := queryer.QueryContext(ctx, spec.Query)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("query owner reconciliation table %s failed", spec.Table)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("close owner reconciliation rows for %s failed", spec.Table)
		}
	}()
	columnIndex := make(map[string]int, len(spec.Columns))
	for index, column := range spec.Columns {
		columnIndex[column] = index
	}
	tenantIndex := columnIndex["tenant_id"]
	groups = make([]candidateGroup, 0)
	for rows.Next() {
		values := make([]any, len(spec.Columns))
		pointers := make([]any, len(values))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("scan owner reconciliation rows for %s failed", spec.Table)
		}
		tenantID := sqlText(values[tenantIndex])
		candidates := make([]Candidate, 0, len(spec.CandidateColumns))
		candidateValues := make([]string, 0, len(spec.CandidateColumns))
		unmappedCandidate := false
		for _, candidateColumn := range spec.CandidateColumns {
			value := sqlText(values[columnIndex[candidateColumn.Name]])
			candidateValues = append(candidateValues, value)
			if value != "" {
				if subject := identities[legacyIdentityKey(tenantID, value)]; subject != "" {
					candidates = append(candidates, Candidate{Source: candidateColumn.Source, Subject: subject})
				} else {
					unmappedCandidate = true
				}
			}
		}
		subject, _ := ResolveCandidates(candidates)
		if unmappedCandidate {
			subject = ""
		}
		if subject != "" {
			groups = append(groups, candidateGroup{TenantID: tenantID, CandidateValues: candidateValues, Subject: subject})
		}
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("iterate owner reconciliation rows for %s failed", spec.Table)
	}
	return groups, nil
}

func validateTableSpec(spec TableSpec) error {
	if !ownerReconcileIdentifier.MatchString(spec.Table) {
		return errors.New("owner reconciliation inventory contains an invalid table identifier")
	}
	query := strings.TrimSpace(spec.Query)
	if query == "" || !strings.EqualFold(strings.TrimSpace(strings.SplitN(query, " ", 2)[0]), "select") {
		return fmt.Errorf("owner reconciliation table %s must use a SELECT query", spec.Table)
	}
	if len(spec.Columns) == 0 {
		return fmt.Errorf("owner reconciliation table %s has no result columns", spec.Table)
	}
	for _, column := range spec.Columns {
		if !ownerReconcileIdentifier.MatchString(column) {
			return errors.New("owner reconciliation inventory contains an invalid result identifier")
		}
	}
	for _, candidate := range spec.CandidateColumns {
		if !ownerReconcileIdentifier.MatchString(candidate.Name) || strings.TrimSpace(candidate.Source) == "" {
			return errors.New("owner reconciliation inventory contains an invalid candidate identifier")
		}
		found := false
		for _, column := range spec.Columns {
			if column == candidate.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("owner reconciliation table %s omitted candidate column %s", spec.Table, candidate.Name)
		}
	}
	if update := strings.TrimSpace(spec.UpdateQuery); update != "" {
		if !strings.EqualFold(strings.TrimSpace(strings.SplitN(update, " ", 2)[0]), "update") || strings.Contains(update, ";") {
			return fmt.Errorf("owner reconciliation table %s has an invalid update query", spec.Table)
		}
	}
	return nil
}

func indexLegacyIdentities(identities []LegacyIdentity) (map[string]string, error) {
	result := make(map[string]string, len(identities))
	for _, identity := range identities {
		tenantID := strings.TrimSpace(identity.TenantID)
		legacyUserID := strings.TrimSpace(identity.LegacyUserID)
		subject := strings.TrimSpace(identity.Subject)
		if tenantID == "" || legacyUserID == "" || subject == "" {
			return nil, errors.New("legacy identity metadata contains an incomplete mapping")
		}
		key := legacyIdentityKey(tenantID, legacyUserID)
		if _, exists := result[key]; exists {
			return nil, errors.New("legacy identity metadata contains a duplicate mapping")
		}
		result[key] = subject
	}
	return result, nil
}

func legacyIdentityKey(tenantID, legacyUserID string) string {
	return tenantID + "\x00" + legacyUserID
}

func scanTableRows(ctx context.Context, rows *sql.Rows, spec TableSpec, identities map[string]string) (findings []Finding, autoRows int64, resultErr error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("close owner reconciliation rows for %s failed", spec.Table)
		}
	}()

	columnIndex := make(map[string]int, len(spec.Columns))
	for index, column := range spec.Columns {
		columnIndex[column] = index
	}
	tenantIndex, tenantOK := columnIndex["tenant_id"]
	countIndex, countOK := columnIndex["row_count"]
	if !tenantOK || !countOK {
		return nil, 0, fmt.Errorf("owner reconciliation table %s omitted tenant_id or row_count", spec.Table)
	}
	for rows.Next() {
		values := make([]any, len(spec.Columns))
		pointers := make([]any, len(values))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, 0, ctxErr
			}
			return nil, 0, fmt.Errorf("scan owner reconciliation rows for %s failed", spec.Table)
		}
		tenantID := sqlText(values[tenantIndex])
		rowCount, err := sqlInt64(values[countIndex])
		if err != nil {
			return nil, 0, fmt.Errorf("scan row count for %s failed", spec.Table)
		}
		candidates := make([]Candidate, 0, len(spec.CandidateColumns))
		fingerprintParts := make([]string, 0, len(spec.CandidateColumns))
		unmappedCandidate := false
		for _, candidateColumn := range spec.CandidateColumns {
			value := sqlText(values[columnIndex[candidateColumn.Name]])
			if value == "" {
				continue
			}
			fingerprintParts = append(fingerprintParts, candidateColumn.Source+"="+value)
			if subject := identities[legacyIdentityKey(tenantID, value)]; subject != "" {
				candidates = append(candidates, Candidate{Source: candidateColumn.Source, Subject: subject})
			} else {
				unmappedCandidate = true
			}
		}
		subject, reason := ResolveCandidates(candidates)
		if unmappedCandidate {
			subject, reason = "", "unmapped_candidate"
		}
		if subject != "" {
			autoRows += rowCount
			continue
		}
		findings = append(findings, Finding{
			Table:             spec.Table,
			TenantFingerprint: shortFingerprint(tenantID),
			OwnerFingerprint:  shortFingerprint(strings.Join(fingerprintParts, ";")),
			Rows:              rowCount,
			Reason:            reason,
		})
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, 0, ctxErr
		}
		return nil, 0, fmt.Errorf("iterate owner reconciliation rows for %s failed", spec.Table)
	}
	return findings, autoRows, nil
}

func sqlText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func sqlInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int32:
		return int64(typed), nil
	case int:
		return int64(typed), nil
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	case string:
		return strconv.ParseInt(typed, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported count type %T", value)
	}
}
