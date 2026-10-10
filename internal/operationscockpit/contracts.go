// Package operationscockpit owns manual period financial facts and versioned
// goals. Store access, identity and platform observations retain their owners.
package operationscockpit

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
)

var (
	ErrInvalid     = errors.New("INVALID_REQUEST")
	ErrForbidden   = errors.New("FORBIDDEN")
	ErrNotFound    = errors.New("NOT_FOUND")
	ErrRevision    = errors.New("REVISION_MISMATCH")
	ErrConflict    = errors.New("IDEMPOTENCY_CONFLICT")
	ErrOverlap     = errors.New("PERIOD_OVERLAP")
	ErrAmountRange = errors.New("AMOUNT_OUT_OF_RANGE")
	ErrUnavailable = errors.New("DEPENDENCY_UNAVAILABLE")
)

const MaxFieldAmount int64 = 1_000_000_000_000
const MaxSafeInteger int64 = 9_007_199_254_740_991

var operatingZone = time.FixedZone("UTC+8", 8*60*60)

func Today(now time.Time) string { return now.In(operatingZone).Format(time.DateOnly) }

func UUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

type Scope struct{ OrganizationID, ActorID string }

func (s Scope) Valid() bool {
	return authidentity.IsBoundedIdentifier(s.OrganizationID) && authidentity.IsBoundedIdentifier(s.ActorID)
}

// Access is returned by the server's current authorizer, never decoded from input.
type Access struct{ GoalsRead, GoalsCreate, GoalsManage, StoresRead, FactsWrite, AlertsRead, AdviceRead bool }

type Period struct {
	Start string `json:"startDate"`
	End   string `json:"endDate"`
}

func parseDate(value string) (time.Time, bool) {
	date, err := time.ParseInLocation(time.DateOnly, value, operatingZone)
	return date, err == nil && date.Year() > 0 && date.Format(time.DateOnly) == value
}

func (p Period) Valid() bool {
	start, ok := parseDate(p.Start)
	end, validEnd := parseDate(p.End)
	return ok && validEnd && !end.Before(start) && end.Sub(start)/(24*time.Hour) < 366
}

func (p Period) Days() int {
	if !p.Valid() {
		return 0
	}
	start, _ := parseDate(p.Start)
	end, _ := parseDate(p.End)
	return int(end.Sub(start)/(24*time.Hour)) + 1
}

func (p Period) Intersects(other Period) bool { return p.Start <= other.End && other.Start <= p.End }
func (p Period) Contains(other Period) bool   { return p.Start <= other.Start && p.End >= other.End }

type Amounts struct {
	Revenue     int64 `json:"revenue"`
	Refunds     int64 `json:"refunds"`
	Procurement int64 `json:"procurement"`
	Logistics   int64 `json:"logistics"`
	Platform    int64 `json:"platform"`
	Advertising int64 `json:"advertising"`
	Other       int64 `json:"other"`
}

func (a Amounts) values() [7]int64 {
	return [7]int64{a.Revenue, a.Refunds, a.Procurement, a.Logistics, a.Platform, a.Advertising, a.Other}
}
func (a Amounts) Valid() bool {
	for _, amount := range a.values() {
		if amount < 0 || amount > MaxFieldAmount {
			return false
		}
	}
	return true
}

type FactInput struct {
	Period  Period  `json:"period"`
	Amounts Amounts `json:"amounts"`
	Note    string  `json:"note"`
}

func (f FactInput) Valid(now time.Time) bool {
	return f.Period.Valid() && f.Period.End < Today(now) && f.Amounts.Valid() && utf8.ValidString(f.Note) && utf8.RuneCountInString(f.Note) <= 1000 && strings.IndexFunc(f.Note, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) < 0
}

type Record struct {
	ID        string    `json:"id"`
	StoreID   string    `json:"storeId"`
	Revision  int64     `json:"revision,string"`
	Period    Period    `json:"period"`
	Amounts   Amounts   `json:"amounts"`
	Note      string    `json:"note"`
	UpdatedBy string    `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type RecordEvidence struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision,string"`
	Period   Period `json:"period"`
}

type GoalConfig struct {
	StoreIDs         []string `json:"storeIds"`
	Frequency        string   `json:"frequency"`
	Period           Period   `json:"period"`
	Profit           int64    `json:"profit"`
	MinimumMarginBPS *int64   `json:"minimumMarginBps"`
	NormalBPS        int64    `json:"normalBps"`
	AttentionBPS     int64    `json:"attentionBps"`
}

func (g GoalConfig) Valid() bool {
	if !g.Period.Valid() || g.Profit <= 0 || g.Profit > MaxFieldAmount || g.AttentionBPS <= 0 || g.AttentionBPS >= g.NormalBPS || g.NormalBPS > 10000 || len(g.StoreIDs) < 1 || len(g.StoreIDs) > 50 || g.MinimumMarginBPS != nil && (*g.MinimumMarginBPS < 0 || *g.MinimumMarginBPS > 10000) {
		return false
	}
	seen := make(map[string]bool, len(g.StoreIDs))
	for _, id := range g.StoreIDs {
		if !UUID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	start, _ := parseDate(g.Period.Start)
	end, _ := parseDate(g.Period.End)
	switch g.Frequency {
	case "day":
		return start.Equal(end)
	case "week":
		return start.Weekday() == time.Monday && start.AddDate(0, 0, 6).Equal(end)
	case "month":
		return start.Day() == 1 && start.AddDate(0, 1, -1).Equal(end)
	default:
		return false
	}
}

type GoalHead struct {
	OrganizationID, ID, CreatorID string
	Revision                      int64
}

func (h GoalHead) CanMaintain(scope Scope, access Access) bool {
	return scope.Valid() && h.OrganizationID == scope.OrganizationID && UUID(h.ID) && h.Revision > 0 && authidentity.IsBoundedIdentifier(h.CreatorID) && access.GoalsRead && (scope.ActorID == h.CreatorID || access.GoalsManage)
}

type HeadMetadata struct {
	GoalID         string `json:"goalId"`
	Revision       string `json:"revision"`
	ScopeValid     bool   `json:"scopeValid"`
	CanReconfigure bool   `json:"canReconfigure"`
}
