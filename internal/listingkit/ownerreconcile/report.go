package ownerreconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Finding struct {
	Table             string `json:"table"`
	TenantFingerprint string `json:"tenant_fingerprint"`
	OwnerFingerprint  string `json:"owner_fingerprint"`
	Rows              int64  `json:"rows"`
	Reason            string `json:"reason"`
}

type Resolution struct {
	Table                string `json:"table"`
	TenantFingerprint    string `json:"tenant_fingerprint"`
	CandidateFingerprint string `json:"candidate_fingerprint"`
	SubjectFingerprint   string `json:"subject_fingerprint"`
	Rows                 int64  `json:"rows"`
}

type ReportSummary struct {
	FindingGroups     int   `json:"finding_groups"`
	SystemOwnedGroups int   `json:"system_owned_groups"`
	AffectedRows      int64 `json:"affected_rows"`
	AutoRows          int64 `json:"auto_rows"`
	UnresolvedRows    int64 `json:"unresolved_rows"`
	SystemOwnedRows   int64 `json:"system_owned_rows"`
}

type Report struct {
	SchemaVersion       int           `json:"schema_version"`
	GeneratedAt         time.Time     `json:"generated_at"`
	ConfigName          string        `json:"config_name"`
	DatabaseName        string        `json:"database_name"`
	ReportFingerprint   string        `json:"report_fingerprint,omitempty"`
	Summary             ReportSummary `json:"summary"`
	Findings            []Finding     `json:"findings"`
	SystemOwnedFindings []Finding     `json:"system_owned_findings,omitempty"`
	Resolutions         []Resolution  `json:"resolutions,omitempty"`
}

func NewReport(configName, databaseName string, findings []Finding, autoRows int64) Report {
	return NewReportWithResolutions(configName, databaseName, findings, autoRows, nil)
}

func NewReportWithResolutions(configName, databaseName string, findings []Finding, autoRows int64, resolutions []Resolution) Report {
	return NewReportWithClassifiedFindings(configName, databaseName, findings, nil, autoRows, resolutions)
}

func NewReportWithClassifiedFindings(configName, databaseName string, findings, systemOwnedFindings []Finding, autoRows int64, resolutions []Resolution) Report {
	copyFindings := append([]Finding(nil), findings...)
	copySystemOwnedFindings := append([]Finding(nil), systemOwnedFindings...)
	copyResolutions := append([]Resolution(nil), resolutions...)
	sortFindings(copyFindings)
	sortFindings(copySystemOwnedFindings)
	sortResolutions(copyResolutions)
	var summary ReportSummary
	summary.AutoRows = autoRows
	for _, finding := range copyFindings {
		summary.FindingGroups++
		summary.AffectedRows += finding.Rows
		summary.UnresolvedRows += finding.Rows
	}
	for _, finding := range copySystemOwnedFindings {
		summary.SystemOwnedGroups++
		summary.SystemOwnedRows += finding.Rows
	}
	summary.AffectedRows += autoRows
	summary.AffectedRows += summary.SystemOwnedRows
	return Report{
		SchemaVersion:       1,
		GeneratedAt:         time.Now().UTC(),
		ConfigName:          filepath.Base(strings.TrimSpace(configName)),
		DatabaseName:        strings.TrimSpace(databaseName),
		Summary:             summary,
		Findings:            copyFindings,
		SystemOwnedFindings: copySystemOwnedFindings,
		Resolutions:         copyResolutions,
	}
}

func (report *Report) SetMetadata(configName, databaseName string) {
	if report == nil {
		return
	}
	report.ConfigName = filepath.Base(strings.TrimSpace(configName))
	report.DatabaseName = strings.TrimSpace(databaseName)
}

func (report *Report) SetFingerprint() error {
	if report == nil {
		return nil
	}
	fingerprint, err := report.Fingerprint()
	if err != nil {
		return err
	}
	report.ReportFingerprint = fingerprint
	return nil
}

func (report Report) Fingerprint() (string, error) {
	findings := append([]Finding(nil), report.Findings...)
	systemOwnedFindings := append([]Finding(nil), report.SystemOwnedFindings...)
	resolutions := append([]Resolution(nil), report.Resolutions...)
	sortFindings(findings)
	sortFindings(systemOwnedFindings)
	sortResolutions(resolutions)
	canonical := struct {
		SchemaVersion       int           `json:"schema_version"`
		Summary             ReportSummary `json:"summary"`
		Findings            []Finding     `json:"findings"`
		SystemOwnedFindings []Finding     `json:"system_owned_findings"`
		Resolutions         []Resolution  `json:"resolutions"`
	}{
		SchemaVersion:       report.SchemaVersion,
		Summary:             report.Summary,
		Findings:            findings,
		SystemOwnedFindings: systemOwnedFindings,
		Resolutions:         resolutions,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:6]), nil
}

func shortFingerprint(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:6])
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.Table != right.Table {
			return left.Table < right.Table
		}
		if left.TenantFingerprint != right.TenantFingerprint {
			return left.TenantFingerprint < right.TenantFingerprint
		}
		if left.OwnerFingerprint != right.OwnerFingerprint {
			return left.OwnerFingerprint < right.OwnerFingerprint
		}
		if left.Reason != right.Reason {
			return left.Reason < right.Reason
		}
		return left.Rows < right.Rows
	})
}

func sortResolutions(resolutions []Resolution) {
	sort.Slice(resolutions, func(i, j int) bool {
		left, right := resolutions[i], resolutions[j]
		if left.Table != right.Table {
			return left.Table < right.Table
		}
		if left.TenantFingerprint != right.TenantFingerprint {
			return left.TenantFingerprint < right.TenantFingerprint
		}
		if left.CandidateFingerprint != right.CandidateFingerprint {
			return left.CandidateFingerprint < right.CandidateFingerprint
		}
		if left.SubjectFingerprint != right.SubjectFingerprint {
			return left.SubjectFingerprint < right.SubjectFingerprint
		}
		return left.Rows < right.Rows
	})
}
