package notificationcenter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

func Digest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func ScopeKey(scope Scope) string { return Digest(scope) }
func RefKey(ref Ref) string       { return Digest(ref) }
func Token(ref Ref) string {
	raw, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(raw)
}
func validText(s string, max int) bool {
	return s != "" && len(s) <= max && strings.TrimSpace(s) == s && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func ValidScope(s Scope) bool {
	u, e := url.Parse(s.Realm)
	return e == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") && len(s.Realm) <= 512 && validText(s.Subject, 128) && (s.OrganizationID == "" || validText(s.OrganizationID, 128)) && (s.Category == Official || s.Category == Business) && (s.Category != Official || s.OrganizationID == "")
}
func ValidKey(key string) bool {
	id, e := uuid.Parse(key)
	return e == nil && id != uuid.Nil && id.String() == key
}
func ParseToken(token string) (Ref, error) {
	var ref Ref
	if len(token) > 2048 {
		return ref, ErrInvalid
	}
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || json.Unmarshal(raw, &ref) != nil || Token(ref) != token || !validRef(ref) {
		return Ref{}, ErrInvalid
	}
	return ref, nil
}
func validRef(r Ref) bool {
	return validText(r.Source, 64) && validText(r.EntityID, 128) && validText(r.Type, 64) && validText(r.Revision, 128) && (r.OrganizationID == "" || validText(r.OrganizationID, 128))
}
func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) validate(scope Scope) error {
	if s == nil || s.Repository == nil {
		return ErrUnavailable
	}
	if !ValidScope(scope) {
		return ErrForbidden
	}
	return nil
}
func (s *Service) source(scope Scope, name string) Source {
	for _, source := range s.Sources {
		if source.Name() == name && source.Category() == scope.Category && (scope.OrganizationID != "" || source.Personal()) {
			return source
		}
	}
	return nil
}
func Href(t Target) (string, error) {
	static := map[string]string{"none": "", "home": "/workbench", "resources": "/workbench/plans/entitlements", "member-resources": "/workbench/account/organization/resources", "members": "/workbench/account/organization/members", "personal-verification": "/workbench/account/profile/verification", "organization-verification": "/workbench/account/organization", "earnings": "/workbench/account/referrals/earnings", "withdrawals": "/workbench/account/referrals/withdrawals"}
	if href, ok := static[t.Kind]; ok && t.ID == "" {
		return href, nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`).MatchString(t.ID) {
		return "", ErrInvalid
	}
	routes := map[string]string{"task": "/workbench/ai/tasks/", "chat": "/workbench/ai/chat/", "acquisition": "/workbench/supply/acquisition/operation/", "store": "/workbench/stores/", "order": "/workbench/plans/orders/", "knowledge": "/workbench/ai/knowledge/", "invitation": "/invitations/"}
	if prefix, ok := routes[t.Kind]; ok {
		return prefix + url.PathEscape(t.ID), nil
	}
	if t.Kind == "review" {
		return "/workbench/ai/tasks/pending/other?proposal_id=" + url.QueryEscape(t.ID), nil
	}
	return "", ErrInvalid
}
func normalize(scope Scope, source Source, item Item) (Item, error) {
	if !validRef(item.Ref) || item.Ref.Source != source.Name() || item.Category != scope.Category || item.Ref.OrganizationID != "" && item.Ref.OrganizationID != scope.OrganizationID || source.Personal() != (item.Ref.OrganizationID == "") || !validText(item.Title, 256) || !validText(item.Summary, 1024) || len(item.Paragraphs) > 32 {
		return Item{}, ErrUnavailable
	}
	size := 0
	for _, p := range item.Paragraphs {
		if !utf8.ValidString(p) {
			return Item{}, ErrUnavailable
		}
		size += len(p)
	}
	if size > 32<<10 {
		return Item{}, ErrUnavailable
	}
	href, e := Href(item.Target)
	if e != nil {
		return Item{}, ErrUnavailable
	}
	if item.OccurredAt != nil && (item.OccurredAt.IsZero() || item.OccurredAt.After(time.Now().Add(time.Minute))) {
		return Item{}, ErrUnavailable
	}
	item.ID = Token(item.Ref)
	item.Type = item.Ref.Type
	item.Source = item.Ref.Source
	item.OrganizationID = item.Ref.OrganizationID
	item.Href = href
	item.Read = false
	if item.Paragraphs == nil {
		item.Paragraphs = []string{}
	}
	return item, nil
}

type collected struct {
	items    []Item
	coverage []Coverage
	exact    bool
}

func (s *Service) collect(ctx context.Context, scope Scope) (collected, error) {
	if e := s.validate(scope); e != nil {
		return collected{}, e
	}
	result := collected{items: []Item{}, coverage: []Coverage{}, exact: true}
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := make(chan struct{}, 4)
	seenNames := map[string]bool{}
	for _, source := range s.Sources {
		if source == nil || seenNames[source.Name()] {
			return collected{}, ErrUnavailable
		}
		seenNames[source.Name()] = true
		if source.Category() != scope.Category || scope.OrganizationID == "" && !source.Personal() {
			continue
		}
		wg.Add(1)
		go func(source Source) {
			defer wg.Done()
			select {
			case workers <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				result.exact = false
				mu.Unlock()
				return
			}
			defer func() { <-workers }()
			sourceCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			coverage := Coverage{Source: source.Name(), State: "AVAILABLE", Complete: true, ObservedAt: s.now()}
			items := []Item{}
			after := ""
			seen := map[string]bool{}
			for {
				page, e := source.ListVisible(sourceCtx, scope, after, 100)
				if e != nil {
					items = []Item{}
					coverage.State = "UNAVAILABLE"
					coverage.Complete = false
					if errors.Is(e, ErrForbidden) {
						coverage.State = "DENIED"
						coverage.Complete = true
					}
					if errors.Is(e, ErrDependencyMissing) {
						coverage.State = "DEPENDENCY_MISSING"
						coverage.Complete = true
					}
					break
				}
				if len(page.Items) > 100 || sourceCtx.Err() != nil {
					coverage.State = "UNAVAILABLE"
					coverage.Complete = false
					items = []Item{}
					break
				}
				for _, raw := range page.Items {
					item, e := normalize(scope, source, raw)
					if e != nil {
						coverage.State = "UNAVAILABLE"
						coverage.Complete = false
						items = []Item{}
						break
					}
					item.Paragraphs = []string{}
					items = append(items, item)
				}
				if !coverage.Complete || page.Next == "" {
					break
				}
				if page.Next == after || seen[page.Next] || len(items) >= MaxItems {
					coverage.State = "UNAVAILABLE"
					coverage.Complete = false
					break
				}
				seen[page.Next] = true
				after = page.Next
			}
			if len(items) == 0 && coverage.State == "AVAILABLE" {
				coverage.State = "EMPTY"
			}
			mu.Lock()
			result.items = append(result.items, items...)
			result.coverage = append(result.coverage, coverage)
			result.exact = result.exact && coverage.Complete
			mu.Unlock()
		}(source)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return collected{}, ErrUnavailable
	}
	taskBindings := map[string]bool{}
	for _, v := range result.items {
		if v.Source == "workbench-task" && v.Association != "" {
			taskBindings[v.Association] = true
		}
	}
	unique := map[string]Item{}
	for _, v := range result.items {
		if v.Source == "product-review" && v.Association != "" && taskBindings[v.Association] {
			continue
		}
		if _, exists := unique[v.ID]; exists {
			return collected{}, ErrUnavailable
		}
		unique[v.ID] = v
	}
	result.items = []Item{}
	for _, v := range unique {
		result.items = append(result.items, v)
	}
	sort.Slice(result.items, func(i, j int) bool {
		a, b := result.items[i], result.items[j]
		if a.OccurredAt == nil && b.OccurredAt != nil {
			return false
		}
		if a.OccurredAt != nil && b.OccurredAt == nil {
			return true
		}
		if a.OccurredAt != nil && !a.OccurredAt.Equal(*b.OccurredAt) {
			return a.OccurredAt.After(*b.OccurredAt)
		}
		return a.ID < b.ID
	})
	sort.Slice(result.coverage, func(i, j int) bool { return result.coverage[i].Source < result.coverage[j].Source })
	raw, _ := json.Marshal(result.items)
	if len(result.items) > MaxItems || len(raw) > MaxCollectionBytes {
		return collected{}, ErrCapacity
	}
	return result, nil
}
func (s *Service) List(ctx context.Context, scope Scope, req ListRequest) (List, error) {
	if req.Limit < 1 || req.Limit > 100 || req.Filter != "" && req.Filter != "all" && req.Filter != "unread" && req.Filter != "pending" || scope.Category == Official && req.Filter == "pending" {
		return List{}, ErrInvalid
	}
	c, e := s.collect(ctx, scope)
	if e != nil {
		return List{}, e
	}
	refs := make([]Ref, len(c.items))
	for i, v := range c.items {
		refs[i] = v.Ref
	}
	reads, e := s.Repository.ReadStates(ctx, scope, refs)
	if e != nil {
		return List{}, e
	}
	out := List{SchemaVersion: SchemaVersion, Items: []Item{}, Coverage: c.coverage, Exact: c.exact, Count: len(c.items)}
	filtered := []Item{}
	for _, item := range c.items {
		item.Paragraphs = []string{}
		item.Read = reads[RefKey(item.Ref)]
		if !item.Read {
			out.Unread++
		}
		if item.Attention {
			out.Pending++
		}
		if req.Filter == "unread" && item.Read || req.Filter == "pending" && !item.Attention {
			continue
		}
		filtered = append(filtered, item)
	}
	start := 0
	if req.After != "" {
		var cursor struct{ Scope, Filter, Last string }
		raw, e := base64.RawURLEncoding.DecodeString(req.After)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Scope != ScopeKey(scope) || cursor.Filter != req.Filter {
			return List{}, ErrInvalid
		}
		found := false
		for i, item := range filtered {
			if item.ID == cursor.Last {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return List{}, ErrStale
		}
	}
	end := start + req.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	// Reserve response metadata space and paginate by actual encoded bytes.
	// A caller may request 100 summary cards without exceeding the HTTP bound.
	pageBytes := 0
	for i := start; i < end; i++ {
		raw, err := json.Marshal(filtered[i])
		if err != nil {
			return List{}, ErrUnavailable
		}
		if pageBytes+len(raw) > 112<<10 {
			end = i
			break
		}
		pageBytes += len(raw) + 1
	}
	out.Items = filtered[start:end]
	if end < len(filtered) {
		raw, _ := json.Marshal(struct{ Scope, Filter, Last string }{ScopeKey(scope), req.Filter, filtered[end-1].ID})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}
func (s *Service) Detail(ctx context.Context, scope Scope, token string) (Item, error) {
	if e := s.validate(scope); e != nil {
		return Item{}, e
	}
	ref, e := ParseToken(token)
	if e != nil {
		return Item{}, e
	}
	if ref.OrganizationID != "" && ref.OrganizationID != scope.OrganizationID {
		return Item{}, ErrNotFound
	}
	source := s.source(scope, ref.Source)
	if source == nil {
		return Item{}, ErrNotFound
	}
	raw, e := source.ReadVisible(ctx, scope, ref)
	if e != nil {
		return Item{}, e
	}
	item, e := normalize(scope, source, raw)
	if e != nil {
		return Item{}, e
	}
	if item.Ref != ref {
		return Item{}, ErrStale
	}
	reads, e := s.Repository.ReadStates(ctx, scope, []Ref{ref})
	if e != nil {
		return Item{}, e
	}
	item.Read = reads[RefKey(ref)]
	return item, nil
}
func (s *Service) command(ctx context.Context, scope Scope, key, op string, payload any) (Command, bool, error) {
	if e := s.validate(scope); e != nil {
		return Command{}, false, e
	}
	if !ValidKey(key) {
		return Command{}, false, ErrInvalid
	}
	c := Command{Key: key, Operation: op, Fingerprint: Digest(payload), CommittedAt: s.now()}
	saved, replay, e := s.Repository.Replay(ctx, scope, key, op, c.Fingerprint)
	if e != nil || replay {
		return saved, replay, e
	}
	return c, false, nil
}
func (s *Service) Read(ctx context.Context, scope Scope, key, token string) (Command, error) {
	ref, e := ParseToken(token)
	if e != nil {
		return Command{}, e
	}
	c, replay, e := s.command(ctx, scope, key, "read", ref)
	if e != nil || replay {
		return c, e
	}
	if _, e = s.Detail(ctx, scope, token); e != nil {
		return Command{}, e
	}
	return s.Repository.CommitRead(ctx, scope, c, []Ref{ref}, "")
}
func (s *Service) PrepareReadAll(ctx context.Context, scope Scope, key string) (Snapshot, error) {
	c, replay, e := s.command(ctx, scope, key, "snapshot", SchemaVersion)
	if e != nil {
		return Snapshot{}, e
	}
	if replay {
		snap, e := s.Repository.Snapshot(ctx, scope, c.ResultID)
		if e != nil {
			return Snapshot{}, e
		}
		if !s.now().Before(snap.ExpiresAt) {
			return Snapshot{}, ErrStale
		}
		return snap, nil
	}
	collected, e := s.collect(ctx, scope)
	if e != nil {
		return Snapshot{}, e
	}
	if !collected.exact {
		return Snapshot{}, ErrUnavailable
	}
	refs := make([]Ref, len(collected.items))
	for i, v := range collected.items {
		refs[i] = v.Ref
	}
	sort.Slice(refs, func(i, j int) bool { return RefKey(refs[i]) < RefKey(refs[j]) })
	c.CommittedAt = s.now()
	snap := Snapshot{ID: uuid.NewString(), Refs: refs, Count: len(refs), ExpiresAt: c.CommittedAt.Add(SnapshotTTL), Fingerprint: Digest(struct {
		Scope Scope
		Refs  []Ref
	}{scope, refs})}
	c.ResultID = snap.ID
	return s.Repository.CreateSnapshot(ctx, scope, c, snap)
}
func (s *Service) ReadAll(ctx context.Context, scope Scope, key, id, fingerprint string) (Command, error) {
	if !ValidKey(id) || len(fingerprint) != 64 {
		return Command{}, ErrInvalid
	}
	c, replay, e := s.command(ctx, scope, key, "read-all", []string{id, fingerprint})
	if e != nil || replay {
		return c, e
	}
	snap, e := s.Repository.Snapshot(ctx, scope, id)
	if e != nil {
		return Command{}, e
	}
	if !s.now().Before(snap.ExpiresAt) || snap.Fingerprint != fingerprint {
		return Command{}, ErrStale
	}
	current, e := s.collect(ctx, scope)
	if e != nil {
		return Command{}, e
	}
	if !current.exact {
		return Command{}, ErrUnavailable
	}
	visible := map[string]bool{}
	for _, item := range current.items {
		visible[RefKey(item.Ref)] = true
	}
	for _, ref := range snap.Refs {
		if !visible[RefKey(ref)] {
			return Command{}, ErrStale
		}
	}
	bySource := map[string][]Ref{}
	for _, ref := range snap.Refs {
		bySource[ref.Source] = append(bySource[ref.Source], ref)
	}
	for _, source := range s.Sources {
		refs := bySource[source.Name()]
		if len(refs) == 0 {
			continue
		}
		gateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		var items []Item
		if batch, ok := source.(ResourceBatchReader); ok {
			items, e = batch.ReadVisibleRefs(gateCtx, scope, refs)
		} else {
			for _, ref := range refs {
				var item Item
				item, e = source.ReadVisible(gateCtx, scope, ref)
				if e != nil {
					break
				}
				items = append(items, item)
			}
		}
		cancel()
		if e != nil {
			return Command{}, e
		}
		checked := map[string]bool{}
		for _, raw := range items {
			item, err := normalize(scope, source, raw)
			if err != nil {
				return Command{}, err
			}
			checked[RefKey(item.Ref)] = true
		}
		for _, ref := range refs {
			if !checked[RefKey(ref)] {
				return Command{}, ErrStale
			}
		}
	}
	return s.Repository.CommitRead(ctx, scope, c, snap.Refs, id)
}
func ValidAnnouncement(input AnnouncementInput) bool {
	if input.Category != "PRODUCT" && input.Category != "SYSTEM" && input.Category != "ACTIVITY" && input.Category != "POLICY" || !validText(input.Title, 256) || !validText(input.Summary, 1024) || len(input.Paragraphs) == 0 || len(input.Paragraphs) > 32 {
		return false
	}
	size := 0
	for _, p := range input.Paragraphs {
		if !utf8.ValidString(p) || strings.TrimSpace(p) == "" {
			return false
		}
		size += len(p)
	}
	_, e := Href(input.Target)
	return size <= 32<<10 && e == nil
}
func (s *Service) Publish(ctx context.Context, scope Scope, key string, input AnnouncementInput) (Command, error) {
	if scope.Category != Official || !ValidAnnouncement(input) {
		return Command{}, ErrInvalid
	}
	c, replay, e := s.command(ctx, scope, key, "publish", input)
	if e != nil || replay {
		return c, e
	}
	return s.Repository.Publish(ctx, scope, c, input)
}
func (s *Service) Withdraw(ctx context.Context, scope Scope, key, id string, revision int64) (Command, error) {
	if scope.Category != Official || !ValidKey(id) || revision != 1 {
		return Command{}, ErrInvalid
	}
	c, replay, e := s.command(ctx, scope, key, "withdraw", struct {
		ID       string
		Revision int64
	}{id, revision})
	if e != nil || replay {
		return c, e
	}
	return s.Repository.Withdraw(ctx, scope, c, id, revision)
}
