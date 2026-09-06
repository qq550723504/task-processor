// Package review owns the bounded title proposal review use case (#333).
package review

import (
	"errors"
	"strings"
	"task-processor/internal/product/enrichment"
	"time"
	"unicode"
	"unicode/utf8"
)

const Timeout = 10 * time.Second
const MaxRecordBytes = 64 << 10

var (
	ErrInvalid     = errors.New("invalid review request")
	ErrForbidden   = errors.New("review permission denied")
	ErrNotFound    = errors.New("review resource not found")
	ErrConflict    = errors.New("review operation conflict")
	ErrUnavailable = errors.New("review unavailable")
	ErrTooLarge    = errors.New("review input too large")
)

type CreateInput struct {
	ProductKey  string `json:"product_key"`
	BaseVersion uint64 `json:"base_version"`
}
type DecisionInput struct {
	Action           string `json:"action"`
	ExpectedRevision uint64 `json:"expected_revision"`
	Title            string `json:"title,omitempty"`
}
type ApplyInput struct {
	ExpectedRevision uint64 `json:"expected_revision"`
}
type Decision struct {
	Action   string    `json:"action"`
	Actor    string    `json:"actor"`
	Revision uint64    `json:"revision"`
	Before   string    `json:"before"`
	After    string    `json:"after"`
	At       time.Time `json:"at"`
}
type Receipt struct {
	ProposalID     string    `json:"proposal_id"`
	Revision       uint64    `json:"revision"`
	ProductVersion uint64    `json:"product_version"`
	PublicationID  string    `json:"publication_id"`
	Actor          string    `json:"actor"`
	At             time.Time `json:"at"`
}
type Record struct {
	ID, Org, Owner                                  string
	Input                                           CreateInput
	BasePublicationID, Policy, Before, Title, State string
	Revision                                        uint64
	Original                                        enrichment.Proposal
	History                                         []Decision
	Receipt                                         *Receipt
}

// View intentionally excludes provider metadata, complete Product and raw output.
type View struct {
	ID            string                  `json:"proposal_id"`
	Owner         string                  `json:"owner"`
	Input         CreateInput             `json:"input"`
	Before        string                  `json:"before"`
	Title         string                  `json:"after"`
	OriginalTitle string                  `json:"original_title"`
	Policy        string                  `json:"policy"`
	State         string                  `json:"state"`
	Revision      uint64                  `json:"revision"`
	Evidence      []Evidence              `json:"evidence"`
	Quality       enrichment.QualityScore `json:"quality"`
	Unresolved    []string                `json:"unresolved"`
	History       []Decision              `json:"decisions"`
	Receipt       *Receipt                `json:"apply_receipt,omitempty"`
}
type Evidence struct {
	ID            string `json:"id"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
	SnapshotID    string `json:"snapshot_id"`
	Checksum      string `json:"checksum"`
}

func (r Record) View() View {
	v := View{ID: r.ID, Owner: r.Owner, Input: r.Input, Before: r.Before, Title: r.Title, Policy: r.Policy, State: r.State, Revision: r.Revision, Quality: r.Original.Quality, History: r.History, Receipt: r.Receipt}
	if len(r.Original.Changes) == 1 {
		v.OriginalTitle = r.Original.Changes[0].Value
	}
	for _, e := range r.Original.Evidence {
		v.Evidence = append(v.Evidence, Evidence{e.ID, e.ReferenceType, e.ReferenceID, e.SnapshotID, e.Checksum})
	}
	for _, w := range r.Original.Warnings {
		v.Unresolved = append(v.Unresolved, w.Code+": "+w.Message)
	}
	for _, w := range r.Original.Rejections {
		v.Unresolved = append(v.Unresolved, w.Code+": "+w.Message)
	}
	return v
}
func ValidKey(s string) bool {
	return s != "" && len(s) <= 128 && s == strings.TrimSpace(s) && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func ValidateTitle(s string) error {
	if len(s) > 4096 {
		return ErrTooLarge
	}
	if s == "" || s != strings.TrimSpace(s) || !utf8.ValidString(s) || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return ErrInvalid
	}
	return nil
}
func (r *Record) Decide(actor string, d DecisionInput) error {
	if d.ExpectedRevision == 0 || d.ExpectedRevision != r.Revision || r.State == "applied" || r.State == "rejected" || r.Revision >= 100 {
		return ErrConflict
	}
	if d.Action != "accept" && d.Action != "reject" && d.Action != "edit" {
		return ErrInvalid
	}
	if d.Action != "edit" && d.Title != "" {
		return ErrInvalid
	}
	before := r.Title
	if d.Action == "edit" {
		if err := ValidateTitle(d.Title); err != nil {
			return err
		}
		r.Title = d.Title
		r.State = "pending"
	} else if d.Action == "accept" {
		r.State = "accepted"
	} else {
		r.State = "rejected"
	}
	r.Revision++
	r.History = append(r.History, Decision{d.Action, actor, r.Revision, before, r.Title, time.Now().UTC()})
	return nil
}
