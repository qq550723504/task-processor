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
	Columns          []string
	CandidateColumns []CandidateColumn
}

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type Repository struct {
	Queryer   Queryer
	Inventory []TableSpec
}

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
		for _, candidateColumn := range spec.CandidateColumns {
			value := sqlText(values[columnIndex[candidateColumn.Name]])
			if value == "" {
				continue
			}
			fingerprintParts = append(fingerprintParts, candidateColumn.Source+"="+value)
			if subject := identities[legacyIdentityKey(tenantID, value)]; subject != "" {
				candidates = append(candidates, Candidate{Source: candidateColumn.Source, Subject: subject})
			}
		}
		subject, reason := ResolveCandidates(candidates)
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
