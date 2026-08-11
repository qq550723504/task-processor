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
	FindingGroups  int   `json:"finding_groups"`
	AffectedRows   int64 `json:"affected_rows"`
	AutoRows       int64 `json:"auto_rows"`
	UnresolvedRows int64 `json:"unresolved_rows"`
}

type Report struct {
	SchemaVersion     int           `json:"schema_version"`
	GeneratedAt       time.Time     `json:"generated_at"`
	ConfigName        string        `json:"config_name"`
	DatabaseName      string        `json:"database_name"`
	ReportFingerprint string        `json:"report_fingerprint,omitempty"`
	Summary           ReportSummary `json:"summary"`
	Findings          []Finding     `json:"findings"`
	Resolutions       []Resolution  `json:"resolutions,omitempty"`
}

func NewReport(configName, databaseName string, findings []Finding, autoRows int64) Report {
	return NewReportWithResolutions(configName, databaseName, findings, autoRows, nil)
}

func NewReportWithResolutions(configName, databaseName string, findings []Finding, autoRows int64, resolutions []Resolution) Report {
	copyFindings := append([]Finding(nil), findings...)
	copyResolutions := append([]Resolution(nil), resolutions...)
	sortFindings(copyFindings)
	sortResolutions(copyResolutions)
	var summary ReportSummary
	summary.AutoRows = autoRows
	for _, finding := range copyFindings {
		summary.FindingGroups++
		summary.AffectedRows += finding.Rows
		summary.UnresolvedRows += finding.Rows
	}
	summary.AffectedRows += autoRows
	return Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		ConfigName:    filepath.Base(strings.TrimSpace(configName)),
		DatabaseName:  strings.TrimSpace(databaseName),
		Summary:       summary,
		Findings:      copyFindings,
		Resolutions:   copyResolutions,
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
	resolutions := append([]Resolution(nil), report.Resolutions...)
	sortFindings(findings)
	sortResolutions(resolutions)
	canonical := struct {
		SchemaVersion int           `json:"schema_version"`
		Summary       ReportSummary `json:"summary"`
		Findings      []Finding     `json:"findings"`
		Resolutions   []Resolution  `json:"resolutions"`
	}{
		SchemaVersion: report.SchemaVersion,
		Summary:       report.Summary,
		Findings:      findings,
		Resolutions:   resolutions,
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
