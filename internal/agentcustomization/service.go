// Package agentcustomization owns service requests and their bounded private deliveries.
package agentcustomization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("CUSTOMIZATION_INVALID")
	ErrForbidden   = errors.New("CUSTOMIZATION_FORBIDDEN")
	ErrNotFound    = errors.New("CUSTOMIZATION_NOT_FOUND")
	ErrConflict    = errors.New("CUSTOMIZATION_CONFLICT")
	ErrRevision    = errors.New("CUSTOMIZATION_REVISION_MISMATCH")
	ErrUnavailable = errors.New("CUSTOMIZATION_UNAVAILABLE")
)

const MaxFileBytes = 2 << 20
const ConsentVersion = "agent-customization-contact-v1"

type Stage string

const (
	Submitted  Stage = "SUBMITTED"
	Evaluating Stage = "EVALUATING"
	Proposed   Stage = "PROPOSED"
	Developing Stage = "DEVELOPING"
	Delivered  Stage = "DELIVERED"
)

var Stages = []Stage{Submitted, Evaluating, Proposed, Developing, Delivered}

type Scope struct {
	OrganizationID, ActorID string
	Platform                bool
}
type Upload struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}
type Input struct {
	Name          string   `json:"name"`
	Scenario      string   `json:"scenario"`
	Direction     string   `json:"direction"`
	Description   string   `json:"description"`
	ContactName   string   `json:"contactName"`
	ContactMethod string   `json:"contactMethod"`
	Consent       bool     `json:"consent"`
	Files         []Upload `json:"files,omitempty"`
}
type Update struct {
	DeliverQualityAgent bool   `json:"deliverQualityAgent,omitempty"`
	Stage               Stage  `json:"stage"`
	Note                string `json:"note"`
	Proposal            string `json:"proposal,omitempty"`
	OfflineConfirmation string `json:"offlineConfirmation,omitempty"`
}
type Attachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}
type Request struct {
	DeliveryID          string       `json:"deliveryId,omitempty"`
	ID                  string       `json:"id"`
	OrganizationID      string       `json:"organizationId"`
	CreatedBy           string       `json:"createdBy"`
	Input               Input        `json:"input"`
	Stage               Stage        `json:"stage"`
	Version             string       `json:"version"`
	Proposal            string       `json:"proposal"`
	OfflineConfirmation string       `json:"offlineConfirmation"`
	ConsentVersion      string       `json:"consentVersion"`
	Attachments         []Attachment `json:"attachments"`
	CreatedAt           time.Time    `json:"createdAt"`
	UpdatedAt           time.Time    `json:"updatedAt"`
}
type Event struct {
	Version string `json:"version"`
	ActorID string `json:"actorId"`
	Update
	At time.Time `json:"at"`
}
type Detail struct {
	Request          Request `json:"request"`
	Events           []Event `json:"events"`
	NextEventVersion string  `json:"nextEventVersion"`
}
type Page struct {
	Items      []Request `json:"items"`
	NextCursor string    `json:"nextCursor"`
}
type Receipt struct {
	RequestID string    `json:"requestId"`
	Key       string    `json:"key"`
	Version   string    `json:"version"`
	Stage     Stage     `json:"stage"`
	At        time.Time `json:"at"`
}
type Command struct {
	Scope              Scope
	Key, ID, Operation string
	Expected           int64
	Input              Input
	Update             Update
	Fingerprint        string
}
type Repository interface {
	Execute(context.Context, Command) (Receipt, error)
	List(context.Context, Scope, string) (Page, error)
	Read(context.Context, Scope, string, int64) (Detail, error)
	Download(context.Context, Scope, string, string) (Attachment, []byte, error)
}
type Service struct{ repo Repository }

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, ErrUnavailable
	}
	return &Service{repo: repo}, nil
}
func UUID(v string) bool { u, e := uuid.Parse(v); return e == nil && u != uuid.Nil && u.String() == v }
func text(v string, max int, multiline bool) bool {
	return v != "" && strings.TrimSpace(v) == v && utf8.ValidString(v) && len(v) <= max*4 && utf8.RuneCountInString(v) <= max && strings.IndexFunc(v, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t' || r == '\r'))
	}) < 0
}
func ValidScope(s Scope) bool {
	return text(s.ActorID, 256, false) && ((s.Platform && s.OrganizationID == "") || (!s.Platform && text(s.OrganizationID, 128, false)))
}
func FileType(data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxFileBytes {
		return "", ErrInvalid
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "application/pdf", "image/png", "image/jpeg":
		return mime, nil
	case "text/plain; charset=utf-8":
		if utf8.Valid(data) {
			return mime, nil
		}
	}
	return "", ErrInvalid
}
func ValidateInput(in Input) error {
	if !in.Consent || !text(in.Name, 120, false) || !text(in.Scenario, 240, false) || !text(in.Description, 10000, true) || !text(in.ContactName, 80, false) || !text(in.ContactMethod, 160, false) || len(in.Files) > 3 {
		return ErrInvalid
	}
	switch in.Direction {
	case "PRODUCT_SUPPLY", "STORE_OPERATIONS", "DATA_ANALYSIS", "OTHER":
	default:
		return ErrInvalid
	}
	for _, f := range in.Files {
		if !text(f.Name, 200, false) || strings.ContainsAny(f.Name, "/\\") {
			return ErrInvalid
		}
		if _, e := FileType(f.Data); e != nil {
			return e
		}
	}
	return nil
}
func ApplyUpdate(r *Request, u Update, platform bool) error {
	if !platform {
		return ErrForbidden
	}
	if r == nil || !text(u.Note, 5000, true) {
		return ErrInvalid
	}
	if u.DeliverQualityAgent && (u.Stage != Delivered || r.Proposal == "" || r.OfflineConfirmation == "") {
		return ErrInvalid
	}
	from, to := -1, -1
	for i, s := range Stages {
		if s == r.Stage {
			from = i
		}
		if s == u.Stage {
			to = i
		}
	}
	if from < 0 || to < from || to > from+1 {
		return ErrConflict
	}
	if u.Proposal != "" && (u.Stage != Proposed || !text(u.Proposal, 5000, true)) {
		return ErrInvalid
	}
	if u.Stage == Proposed && r.Stage != Proposed && u.Proposal == "" {
		return ErrInvalid
	}
	if u.OfflineConfirmation != "" && (r.Stage != Proposed || u.Stage != Developing || !text(u.OfflineConfirmation, 5000, true)) {
		return ErrInvalid
	}
	if r.Stage == Proposed && u.Stage == Developing && (r.Proposal == "" || u.OfflineConfirmation == "") {
		return ErrInvalid
	}
	if u.Proposal != "" {
		r.Proposal = u.Proposal
	}
	if u.OfflineConfirmation != "" {
		r.OfflineConfirmation = u.OfflineConfirmation
	}
	r.Stage = u.Stage
	return nil
}
func (s *Service) Execute(ctx context.Context, c Command) (Receipt, error) {
	if !ValidScope(c.Scope) || !UUID(c.Key) {
		return Receipt{}, ErrInvalid
	}
	if c.Operation == "submit" {
		if c.Scope.Platform {
			return Receipt{}, ErrForbidden
		}
		if c.ID != "" || c.Expected != 0 {
			return Receipt{}, ErrInvalid
		}
		if e := ValidateInput(c.Input); e != nil {
			return Receipt{}, e
		}
	} else if c.Operation == "progress" {
		if !c.Scope.Platform {
			return Receipt{}, ErrForbidden
		}
		if !UUID(c.ID) || c.Expected < 1 || !text(c.Update.Note, 5000, true) {
			return Receipt{}, ErrInvalid
		}
	} else {
		return Receipt{}, ErrInvalid
	}
	c.Fingerprint = ""
	raw, e := json.Marshal(c)
	if e != nil {
		return Receipt{}, ErrInvalid
	}
	digest := sha256.Sum256(raw)
	c.Fingerprint = hex.EncodeToString(digest[:])
	return s.repo.Execute(ctx, c)
}
func (s *Service) List(ctx context.Context, scope Scope, cursor string) (Page, error) {
	if !ValidScope(scope) || (cursor != "" && !UUID(cursor)) {
		return Page{}, ErrInvalid
	}
	return s.repo.List(ctx, scope, cursor)
}
func (s *Service) Read(ctx context.Context, scope Scope, id string, after int64) (Detail, error) {
	if !ValidScope(scope) || !UUID(id) || after < 0 {
		return Detail{}, ErrInvalid
	}
	return s.repo.Read(ctx, scope, id, after)
}
func (s *Service) Download(ctx context.Context, scope Scope, id, file string) (Attachment, []byte, error) {
	if !ValidScope(scope) || !UUID(id) || !UUID(file) {
		return Attachment{}, nil, ErrInvalid
	}
	return s.repo.Download(ctx, scope, id, file)
}
