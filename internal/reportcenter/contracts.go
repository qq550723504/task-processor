// Package reportcenter owns personal historical presentation snapshots only.
package reportcenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	"time"
	"unicode"
	"unicode/utf8"
)

const ReadPermission = "workbench.report.read"
const ManagePermission = "workbench.report.manage"
const MaxContentBytes = 128 << 10

var (
	ErrInvalid     = errors.New("INVALID_REQUEST")
	ErrForbidden   = errors.New("FORBIDDEN")
	ErrNotFound    = errors.New("NOT_FOUND")
	ErrConflict    = errors.New("CONFLICT")
	ErrUnavailable = errors.New("DEPENDENCY_UNAVAILABLE")
)

type Scope struct{ OrganizationID, ActorID string }

func (s Scope) Valid() bool {
	return authidentity.IsBoundedIdentifier(s.OrganizationID) && authidentity.IsBoundedIdentifier(s.ActorID)
}
func UUID(s string) bool { u, e := uuid.Parse(s); return e == nil && u != uuid.Nil && u.String() == s }

type SourceRef struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

func KindValid(k string) bool { return k == "TITLE_REVIEW" || k == "SHEIN_RECORD" }
func (r SourceRef) Valid() bool {
	if !KindValid(r.Kind) || !UUID(r.ID) {
		return false
	}
	if r.Kind == "SHEIN_RECORD" {
		value, canonical := strings.CutPrefix(r.Version, "sha256:")
		b, e := hex.DecodeString(value)
		return canonical && e == nil && len(b) == 32 && hex.EncodeToString(b) == value
	}
	p := strings.Split(r.Version, ":")
	if len(p) != 2 {
		return false
	}
	n, e := strconv.ParseUint(p[0], 10, 63)
	return e == nil && n > 0 && strconv.FormatUint(n, 10) == p[0] && (p[1] == "pending" || p[1] == "accepted" || p[1] == "rejected" || p[1] == "applied")
}

type Field struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type Section struct {
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}
type Document struct {
	SchemaVersion int       `json:"schemaVersion"`
	Sections      []Section `json:"sections"`
}
type SourceInfo struct {
	Ref        SourceRef  `json:"ref"`
	Title      string     `json:"title"`
	ProductKey string     `json:"productKey"`
	StoreID    string     `json:"storeId"`
	SourceAt   *time.Time `json:"sourceAt,omitempty"`
}
type Snapshot struct {
	SourceInfo
	Content Document `json:"content"`
}

func textValid(s string, limit int, multiline bool) bool {
	return utf8.ValidString(s) && len(s) <= limit && (multiline || strings.TrimSpace(s) != "") && strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))
	}) < 0
}
func (s Snapshot) Bytes() ([]byte, string, error) {
	if !s.Ref.Valid() || !textValid(s.Title, 256, false) || !textValid(s.ProductKey, 128, false) || (s.StoreID != "" && !UUID(s.StoreID)) || (s.SourceAt != nil && s.SourceAt.IsZero()) || s.Content.SchemaVersion != 1 || len(s.Content.Sections) == 0 || len(s.Content.Sections) > 16 {
		return nil, "", ErrUnavailable
	}
	fields := 0
	for _, section := range s.Content.Sections {
		if !textValid(section.Title, 256, false) || len(section.Fields) == 0 {
			return nil, "", ErrUnavailable
		}
		fields += len(section.Fields)
		for _, f := range section.Fields {
			if !textValid(f.Label, 256, false) || !textValid(f.Value, 16000, true) {
				return nil, "", ErrUnavailable
			}
		}
	}
	if fields > 256 {
		return nil, "", ErrUnavailable
	}
	b, e := json.Marshal(s.Content)
	if e != nil || len(b) > MaxContentBytes {
		return nil, "", ErrUnavailable
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

type ReportSummary struct {
	ID string `json:"id"`
	SourceInfo
	CapturedAt time.Time `json:"capturedAt"`
	Favorite   bool      `json:"favorite"`
}
type Report struct {
	ReportSummary
	Content Document `json:"content"`
	Digest  string   `json:"digest"`
}
type Result struct {
	CommandID string `json:"commandId"`
	Report    Report `json:"report"`
	Replayed  bool   `json:"replayed"`
}
type Filter struct {
	View, Kind, Search, Cursor string
	Limit                      int
}
type Page struct {
	Items      []ReportSummary `json:"items"`
	NextCursor string          `json:"nextCursor"`
}
type Summary struct {
	Saved     int64 `json:"saved"`
	Recent    int64 `json:"recent"`
	Favorites int64 `json:"favorites"`
	Stores    int64 `json:"stores"`
}

func (f Filter) Valid() bool {
	return (f.View == "all" || f.View == "recent" || f.View == "favorites") && (f.Kind == "" || KindValid(f.Kind)) && len(f.Search) <= 128 && utf8.ValidString(f.Search) && strings.IndexFunc(f.Search, unicode.IsControl) < 0 && f.Limit > 0 && f.Limit <= 50 && len(f.Cursor) <= 512
}

type SourceReader interface {
	Read(context.Context, Scope, string, string) (Snapshot, error)
}
type Repository interface {
	Lookup(context.Context, Scope, string, string) (string, error)
	Save(context.Context, Scope, string, string, Snapshot, func(context.Context) error) (Result, error)
	Favorite(context.Context, Scope, string, string, string, bool, func(context.Context) error) (Result, error)
	Read(context.Context, Scope, string) (Report, error)
	List(context.Context, Scope, Filter) (Page, error)
	Summary(context.Context, Scope) (Summary, error)
}

// Authorize returns the currently resolved identity context for source readers.
type Authorize func(context.Context, Scope, bool) (context.Context, error)
type Service struct {
	Repository Repository
	Sources    SourceReader
	Authorize  Authorize
}

func Fingerprint(operation string, intent any) string {
	b, _ := json.Marshal(struct {
		Operation string
		Intent    any
	}{operation, intent})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
