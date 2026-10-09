package projectcenter

import (
	"context"
	"github.com/google/uuid"
	"strings"
	"task-processor/internal/authidentity"
	"time"
	"unicode"
	"unicode/utf8"
)

func ValidID(v string) bool {
	id, e := uuid.Parse(v)
	return e == nil && id.String() == v && id != uuid.Nil
}
func validScope(s Scope) bool {
	return authidentity.IsBoundedIdentifier(s.OrganizationID) && authidentity.IsBoundedIdentifier(s.ActorID)
}
func text(v string, max int, required bool) bool {
	if !utf8.ValidString(v) || len(v) > max || required && strings.TrimSpace(v) == "" {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
func ValidKind(k string) bool {
	switch k {
	case "STORE_OPERATIONS", "PRODUCT_DEVELOPMENT", "PRODUCT_RESEARCH", "BRAND_BUILDING", "OPC", "OTHER":
		return true
	}
	return false
}
func ValidateFields(f Fields) error {
	if !text(f.Title, 256, true) || !text(f.Goal, 4096, true) || !ValidKind(f.Kind) {
		return ErrInvalid
	}
	if f.DueDate != "" {
		t, e := time.Parse("2006-01-02", f.DueDate)
		if e != nil || t.Format("2006-01-02") != f.DueDate {
			return ErrInvalid
		}
	}
	return nil
}
func ValidReference(r Reference) bool {
	if !ValidID(r.TargetID) {
		return false
	}
	switch r.Kind {
	case "CONVERSATION", "BUSINESS_TASK", "PRODUCT", "KNOWLEDGE_BASE", "KNOWLEDGE_SOURCE":
		return true
	}
	return false
}
func validateCommand(c Command) error {
	if c.StoreID != nil && *c.StoreID != "" && !authidentity.IsBoundedIdentifier(*c.StoreID) {
		return ErrInvalid
	}
	switch c.Operation {
	case "create":
		if c.ID != "" || c.Expected != 0 || c.Fields == nil {
			return ErrInvalid
		}
		return ValidateFields(*c.Fields)
	case "edit":
		if !ValidID(c.ID) || c.Expected == 0 || c.Fields == nil {
			return ErrInvalid
		}
		return ValidateFields(*c.Fields)
	case "archive", "restore", "remove", "template_archive":
		if !ValidID(c.ID) || c.Expected == 0 {
			return ErrInvalid
		}
		if c.Operation == "remove" && !ValidID(c.SlotID) {
			return ErrInvalid
		}
	case "add":
		if !ValidID(c.ID) || c.Expected == 0 || c.Reference == nil || c.Reference.SlotID != "" || !ValidReference(*c.Reference) {
			return ErrInvalid
		}
	case "visit":
		if !ValidID(c.ID) || c.Expected != 0 {
			return ErrInvalid
		}
	case "template_save":
		if !ValidID(c.ID) || c.Expected == 0 || !text(c.Name, 256, true) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
func (s *Service) authorized(ctx context.Context, scope Scope, manage bool) error {
	if s == nil || s.Store == nil || s.Authorize == nil {
		return ErrUnavailable
	}
	if ctx == nil || ctx.Err() != nil || !validScope(scope) {
		return ErrInvalid
	}
	return s.Authorize(ctx, scope, manage)
}
func (s *Service) Execute(ctx context.Context, scope Scope, key string, c Command) (Receipt, error) {
	if e := s.authorized(ctx, scope, c.Operation != "visit"); e != nil {
		return Receipt{}, e
	}
	if !ValidID(key) {
		return Receipt{}, ErrInvalid
	}
	if e := validateCommand(c); e != nil {
		return Receipt{}, e
	}
	if out, found, e := s.Store.Replay(ctx, scope, key, c); e != nil || found {
		return out, e
	}
	var validation error
	var ref *Reference
	if c.Operation == "add" {
		ref = c.Reference
	}
	if (c.Operation == "create" || c.Operation == "edit") && c.StoreID != nil && *c.StoreID != "" {
		ref = &Reference{Kind: "STORE", TargetID: *c.StoreID}
	}
	if ref != nil {
		if s.Reader == nil {
			validation = ErrUnavailable
		} else {
			v, e := s.Reader.Resolve(ctx, scope, *ref)
			if e != nil || !v.Available || v.TargetID != ref.TargetID || v.Kind != ref.Kind {
				validation = ErrNotFound
			}
		}
	}
	// Commit takes the scoped key lock and replays a concurrent winner before
	// returning the preflight failure, following the delivered Chat receipt pattern.
	return s.Store.Commit(ctx, scope, key, c, validation)
}
func (s *Service) resolve(ctx context.Context, scope Scope, r Reference) ReferenceView {
	fallback := ReferenceView{SlotID: r.SlotID, Kind: r.Kind}
	if s.Reader == nil {
		return fallback
	}
	v, e := s.Reader.Resolve(ctx, scope, r)
	if e != nil || !v.Available || v.TargetID != r.TargetID || v.Kind != r.Kind || !text(v.Title, 512, true) || !strings.HasPrefix(v.Href, "/workbench/") || strings.ContainsAny(v.Href, "\r\n") {
		return fallback
	}
	v.SlotID = r.SlotID
	if v.ResultHref != "" && (!strings.HasPrefix(v.ResultHref, "/workbench/") || strings.ContainsAny(v.ResultHref, "\r\n")) {
		v.ResultHref = ""
	}
	return v
}
func (s *Service) Get(ctx context.Context, scope Scope, id string) (View, error) {
	if e := s.authorized(ctx, scope, false); e != nil {
		return View{}, e
	}
	if !ValidID(id) {
		return View{}, ErrInvalid
	}
	p, refs, e := s.Store.Get(ctx, scope, id)
	if e != nil {
		return View{}, e
	}
	if p.Scope != scope || p.ID != id || len(refs) > 100 {
		return View{}, ErrUnavailable
	}
	v := View{Project: p, StoreScope: p.StoreID != "", References: []ReferenceView{}, TaskSummaryAvailable: true}
	if p.StoreID != "" {
		store := s.resolve(ctx, scope, Reference{Kind: "STORE", TargetID: p.StoreID})
		v.Store = &store
	}
	for _, r := range refs {
		resolved := s.resolve(ctx, scope, r)
		v.References = append(v.References, resolved)
		if r.Kind == "BUSINESS_TASK" {
			v.TaskTotal++
			if !resolved.Available || resolved.TaskState == "" {
				v.TaskSummaryAvailable = false
			}
			if resolved.TaskState == "COMPLETED" {
				v.TaskCompleted++
			}
			if resolved.TaskState == "WAITING_CONFIRMATION" {
				v.TaskPending++
			}
		}
	}
	if !v.TaskSummaryAvailable {
		v.TaskCompleted = 0
		v.TaskPending = 0
	}
	return v, nil
}
func (s *Service) List(ctx context.Context, scope Scope, q Query) (Page, error) {
	if e := s.authorized(ctx, scope, false); e != nil {
		return Page{}, e
	}
	if q.Mode != "active" && q.Mode != "archived" && q.Mode != "recent" || q.Kind != "" && !ValidKind(q.Kind) || !text(q.Search, 128, false) || q.WorkScope != "" && q.WorkScope != "store" && q.WorkScope != "general" || len(q.After) > 256 || q.StoreID != "" && !authidentity.IsBoundedIdentifier(q.StoreID) {
		return Page{}, ErrInvalid
	}
	projects, next, e := s.Store.List(ctx, scope, q)
	if e != nil {
		return Page{}, e
	}
	if len(projects) > 20 {
		return Page{}, ErrUnavailable
	}
	out := Page{Projects: []View{}, Next: next}
	for _, p := range projects {
		v, e := s.Get(ctx, scope, p.ID)
		if e != nil {
			return Page{}, e
		}
		// Cards consume only the authorized aggregate. Detailed links belong to
		// the single-project view, keeping a full page within the response budget.
		v.References = []ReferenceView{}
		out.Projects = append(out.Projects, v)
	}
	return out, nil
}
func (s *Service) Templates(ctx context.Context, scope Scope, after string) (TemplatePage, error) {
	if e := s.authorized(ctx, scope, false); e != nil {
		return TemplatePage{}, e
	}
	if len(after) > 256 {
		return TemplatePage{}, ErrInvalid
	}
	rows, next, e := s.Store.Templates(ctx, scope, after)
	if e != nil {
		return TemplatePage{}, e
	}
	if len(rows) > 20 {
		return TemplatePage{}, ErrUnavailable
	}
	if rows == nil {
		rows = []Template{}
	}
	for _, r := range rows {
		if r.Scope != scope {
			return TemplatePage{}, ErrUnavailable
		}
	}
	return TemplatePage{Templates: rows, Next: next}, nil
}
