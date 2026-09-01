package enrichment

import (
	"time"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type Request struct {
	Snapshot catalog.ProductSnapshot
	Source   sourcing.SourceEnvelope
	Policy   PolicySnapshot
}

type PolicySnapshot struct {
	Version             string
	AllowedFields       []string
	RequiredFields      []string
	MinimumQualityScore float64
}

type Proposal struct {
	Changes    []FieldChange
	Evidence   []Evidence
	Quality    QualityScore
	Validation ValidationResult
	Warnings   []Warning
	Rejections []Rejection
}

type Candidate struct {
	Changes    []FieldChange
	Warnings   []Warning
	Rejections []Rejection
}

type FieldChange struct {
	Field       string
	Value       string
	EvidenceIDs []string
}

type Evidence struct {
	ReferenceType string
	ID            string
	ReferenceID   string
	SnapshotID    string
	Checksum      string
	URL           string
	CapturedAt    time.Time
	Metadata      map[string]string
}

type QualityScore struct {
	Overall               float64
	EvidenceCoverage      float64
	RequiredFieldCoverage float64
}

type ValidationResult struct {
	Valid            bool
	EvaluatedChanges int
}

type Warning struct {
	Code     string
	Field    string
	Message  string
	Metadata map[string]string
}

type Rejection struct {
	Code     string
	Field    string
	Message  string
	Metadata map[string]string
}
