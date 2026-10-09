package observations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"task-processor/internal/authidentity"
	"time"
)

type Service struct {
	Repository Repository
	Access     Access
	Directory  Directory
	Starter    Starter
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) available() bool { return s != nil && s.Repository != nil && s.Access != nil }
func (s *Service) allowedStores(ctx context.Context, scope Scope, kind Kind, sync bool, selected []string) ([]string, error) {
	if !s.available() || s.Directory == nil {
		return nil, ErrUnavailable
	}
	if !scope.Valid() || !kind.Valid() {
		return nil, ErrInvalid
	}
	if e := s.Access.Authorize(ctx, scope, kind, sync); e != nil {
		return nil, e
	}
	all, e := s.Directory.ListStores(ctx, scope)
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	for _, id := range all {
		if !ValidID(id) || set[id] || len(set) >= 500 {
			return nil, ErrUnavailable
		}
		set[id] = true
	}
	if len(selected) == 0 {
		selected = all
	}
	if len(selected) > 500 {
		return nil, ErrInvalid
	}
	out := []string{}
	seen := map[string]bool{}
	for _, id := range selected {
		if !ValidID(id) || seen[id] {
			return nil, ErrInvalid
		}
		if !set[id] {
			return nil, ErrNotFound
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
func (s *Service) Begin(ctx context.Context, scope Scope, key string, in BeginInput) (Command, error) {
	if !s.available() || !scope.Valid() || !in.Kind.Valid() || !ValidID(key) || len(in.Stores) > 500 {
		return Command{}, ErrInvalid
	}
	in.Stores = append([]string{}, in.Stores...)
	sort.Strings(in.Stores)
	for i, id := range in.Stores {
		if !ValidID(id) || i > 0 && in.Stores[i-1] == id {
			return Command{}, ErrInvalid
		}
	}
	if in.Start != nil {
		x := in.Start.UTC().Truncate(time.Second)
		in.Start = &x
	}
	if in.End != nil {
		x := in.End.UTC().Truncate(time.Second)
		in.End = &x
	}
	if in.Kind == Products && (in.Start != nil || in.End != nil) {
		return Command{}, ErrInvalid
	}
	raw, _ := json.Marshal(in)
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	if e := s.Access.Authorize(ctx, scope, in.Kind, true); e != nil {
		return Command{}, e
	}
	prior, e := s.Repository.CommandByKey(ctx, scope, key)
	if e == nil {
		if prior.Hash != fingerprint {
			return Command{}, ErrConflict
		}
		return s.command(ctx, scope, prior)
	}
	if !errors.Is(e, ErrNotFound) {
		return Command{}, e
	}
	stores, e := s.allowedStores(ctx, scope, in.Kind, true, in.Stores)
	if e != nil {
		return Command{}, e
	}
	if len(stores) == 0 {
		return Command{}, ErrInvalid
	}
	fixed := in
	fixed.Stores = stores
	now := s.now().Truncate(time.Second)
	windows := []Window{}
	if in.Kind == Orders {
		end := now
		if fixed.End != nil {
			end = *fixed.End
		}
		start := end.Add(-30 * 24 * time.Hour)
		if fixed.Start != nil {
			start = *fixed.Start
		}
		if end.After(now) {
			return Command{}, ErrInvalid
		}
		windows, e = OrderWindows(start, end)
		if e != nil {
			return Command{}, e
		}
		fixed.Start = &start
		fixed.End = &end
	}
	children := []Sync{}
	for _, store := range stores {
		child := Sync{ID: uuid.NewString(), StoreID: store, Owner: scope, Kind: in.Kind, Key: ChildKey(key, store, in.Kind), Revision: 1, Status: "pending", CreatedAt: now, Progress: Checkpoint{Page: 1, Windows: append([]Window{}, windows...), Notes: []string{}}}
		if in.Kind == Orders {
			child.Range = &Window{*fixed.Start, *fixed.End}
		}
		m, e := s.Access.Open(ctx, scope, store, in.Kind, true, nil)
		if e != nil {
			if !errors.Is(e, ErrUnsupported) && !errors.Is(e, ErrForbidden) && !errors.Is(e, ErrConflict) {
				return Command{}, e
			}
			child.Status = "suspended"
			child.ErrorCode = "store_unavailable"
			if errors.Is(e, ErrUnsupported) {
				child.ErrorCode = "unsupported_application"
			}
		} else {
			child.Binding = m.Binding()
		}
		children = append(children, child)
	}
	cmd, e := s.Repository.Begin(ctx, scope, key, fingerprint, fixed, children)
	if e != nil {
		return Command{}, e
	}
	// A committed pending command stays discoverable even when Temporal startup is
	// unavailable or its response is lost. No second identity is generated.
	if s.Starter != nil {
		for _, child := range cmd.Syncs {
			if !child.Terminal() {
				_ = s.Starter.Ensure(ctx, scope.OrganizationID, child.ID)
			}
		}
	}
	return s.command(ctx, scope, cmd)
}
func (s *Service) command(ctx context.Context, scope Scope, cmd Command) (Command, error) {
	if cmd.Owner != scope {
		return Command{}, ErrNotFound
	}
	if e := s.Access.Authorize(ctx, scope, cmd.Input.Kind, false); e != nil {
		return Command{}, e
	}
	allowed, e := s.allowedStores(ctx, scope, cmd.Input.Kind, false, nil)
	if e != nil {
		return Command{}, e
	}
	set := map[string]bool{}
	for _, id := range allowed {
		set[id] = true
	}
	visible := make([]Sync, 0, len(cmd.Syncs))
	stores := make([]string, 0, len(cmd.Syncs))
	for _, child := range cmd.Syncs {
		if !set[child.StoreID] {
			continue
		}
		if child.Binding.ApplicationID != "" {
			if _, e := s.Access.Open(ctx, scope, child.StoreID, child.Kind, false, &child.Binding); e != nil {
				if errors.Is(e, ErrNotFound) || errors.Is(e, ErrForbidden) || errors.Is(e, ErrConflict) || errors.Is(e, ErrUnsupported) {
					continue
				}
				return Command{}, e
			}
		}
		visible = append(visible, child)
		stores = append(stores, child.StoreID)
	}
	if len(visible) == 0 {
		return Command{}, ErrNotFound
	}
	// This is a current-access projection, not a rewrite of the immutable parent
	// receipt. Its original hash and selected set still govern same-key replay.
	cmd.Syncs = visible
	cmd.Input.Stores = stores
	return cmd, nil
}
func (s *Service) Command(ctx context.Context, scope Scope, key string) (Command, error) {
	if !s.available() || !scope.Valid() || !ValidID(key) {
		return Command{}, ErrInvalid
	}
	c, e := s.Repository.CommandByKey(ctx, scope, key)
	if e != nil {
		return c, e
	}
	return s.command(ctx, scope, c)
}
func (s *Service) Status(ctx context.Context, scope Scope, id string) (Sync, error) {
	if !s.available() || !scope.Valid() || !ValidID(id) {
		return Sync{}, ErrInvalid
	}
	sync, e := s.Repository.ReadSync(ctx, scope.OrganizationID, id)
	if e != nil {
		return sync, e
	}
	_, e = s.allowedStores(ctx, scope, sync.Kind, false, []string{sync.StoreID})
	if e != nil {
		return Sync{}, e
	}
	if sync.Binding.ApplicationID != "" {
		if _, e = s.Access.Open(ctx, scope, sync.StoreID, sync.Kind, false, &sync.Binding); e != nil {
			return Sync{}, e
		}
	}
	return sync, nil
}
func (s *Service) Ensure(ctx context.Context, scope Scope, id string) (Sync, error) {
	sync, e := s.Status(ctx, scope, id)
	if e != nil {
		return sync, e
	}
	if sync.Owner != scope {
		return Sync{}, ErrNotFound
	}
	if e = s.Access.Authorize(ctx, scope, sync.Kind, true); e != nil {
		return Sync{}, e
	}
	if sync.Terminal() {
		return sync, nil
	}
	if _, e = s.Access.Open(ctx, scope, sync.StoreID, sync.Kind, true, &sync.Binding); e != nil {
		return Sync{}, e
	}
	if s.now().Sub(sync.CreatedAt) >= 24*time.Hour {
		c := copyCheckpoint(sync.Progress)
		c.Note("duration_limit")
		return s.Repository.CommitPage(ctx, sync, c, nil, "partial", s.now())
	}
	if s.Starter == nil {
		return sync, ErrUnavailable
	}
	if e = s.Starter.Ensure(ctx, scope.OrganizationID, id); e != nil {
		return sync, ErrUnavailable
	}
	return sync, nil
}
func (s *Service) Step(ctx context.Context, org, id string) (Sync, error) {
	if !s.available() || !authidentity.IsBoundedIdentifier(org) || !ValidID(id) {
		return Sync{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	sync, e := s.Repository.ReadSync(ctx, org, id)
	if e != nil {
		return sync, e
	}
	if sync.Owner.OrganizationID != org || !sync.Owner.Valid() {
		return Sync{}, ErrForbidden
	}
	if sync.Terminal() {
		return sync, nil
	}
	stop := func(e error) (Sync, error) {
		if errors.Is(e, ErrForbidden) || errors.Is(e, ErrConflict) || errors.Is(e, ErrUnsupported) {
			return s.Repository.Stop(ctx, sync, "suspended", "access_or_source_changed")
		}
		return Sync{}, e
	}
	if e = s.Access.Authorize(ctx, sync.Owner, sync.Kind, true); e != nil {
		return stop(e)
	}
	if s.now().Sub(sync.CreatedAt) >= 24*time.Hour {
		c := copyCheckpoint(sync.Progress)
		c.Note("duration_limit")
		return s.Repository.CommitPage(ctx, sync, c, nil, "partial", s.now())
	}
	merchant, e := s.Access.Open(ctx, sync.Owner, sync.StoreID, sync.Kind, true, &sync.Binding)
	if e != nil {
		return stop(e)
	}
	if merchant.Binding() != sync.Binding {
		return stop(ErrConflict)
	}
	c := copyCheckpoint(sync.Progress)
	records := []Record{}
	count := 0
	var total *int
	window := ""
	if sync.Kind == Products {
		page, e := merchant.Products(ctx, c.Page)
		if e != nil {
			return stop(e)
		}
		count = len(page.Items)
		total = &page.Total
		seen := map[string]bool{}
		for _, p := range page.Items {
			if !Identity(p.ID) || seen[p.ID] {
				return Sync{}, ErrUnavailable
			}
			seen[p.ID] = true
			for _, skc := range p.SKCs {
				if skc.Site == "" || skc.SiteStatus == nil {
					c.Note("product_site_unknown")
				}
			}
			product := p
			records = append(records, Record{StoreID: sync.StoreID, SyncID: sync.ID, ID: p.ID, Product: &product})
		}
	} else {
		if len(c.Windows) == 0 {
			return Sync{}, ErrConflict
		}
		w := c.Windows[0]
		window = w.Key()
		page, e := merchant.Orders(ctx, w, c.Page)
		if errors.Is(e, ErrWindowLimit) {
			next, status := SplitWindow(c)
			if e = merchant.Check(ctx); e != nil {
				return stop(e)
			}
			return s.Repository.CommitPage(ctx, sync, next, nil, status, s.now())
		}
		if e != nil {
			return stop(e)
		}
		count = len(page.Items)
		total = page.ReportedCount
		ids := []string{}
		refs := map[string]OrderRef{}
		for _, ref := range page.Items {
			if !Identity(ref.ID) {
				return Sync{}, ErrUnavailable
			}
			if _, ok := refs[ref.ID]; ok {
				return Sync{}, ErrUnavailable
			}
			refs[ref.ID] = ref
			ids = append(ids, ref.ID)
		}
		if len(ids) > 0 {
			merchant, e = s.Access.Open(ctx, sync.Owner, sync.StoreID, sync.Kind, true, &sync.Binding)
			if e != nil {
				return stop(e)
			}
			details, e := merchant.OrderDetails(ctx, ids)
			if e != nil {
				return stop(e)
			}
			if len(details) != len(ids) {
				return Sync{}, ErrUnavailable
			}
			seen := map[string]bool{}
			for _, detail := range details {
				ref, ok := refs[detail.ID]
				if !ok || seen[detail.ID] {
					return Sync{}, ErrUnavailable
				}
				seen[detail.ID] = true
				if detail.Site == "" {
					c.Note("order_site_missing")
					continue
				}
				if detail.Site != "shein-us" {
					continue
				}
				detail.IssuedAt = ref.CreatedAt.UTC().Format(time.RFC3339)
				if detail.UpdatedAt == "" {
					detail.UpdatedAt = ref.UpdatedAt.UTC().Format(time.RFC3339)
				}
				if detail.Status == nil {
					status := ref.Status
					detail.Status = &status
				}
				records = append(records, Record{StoreID: sync.StoreID, SyncID: sync.ID, ID: detail.ID, Order: &detail, WindowKey: window})
			}
		}
	}
	if e = merchant.Check(ctx); e != nil {
		return stop(e)
	}
	sync.Progress = c
	next, status, e := Advance(sync, count, total)
	if e != nil {
		return Sync{}, e
	}
	at := s.now()
	for i := range records {
		records[i].ObservedAt = at
	}
	return s.Repository.CommitPage(ctx, sync, next, records, status, at)
}
func (s *Service) Fail(ctx context.Context, org, id string) (Sync, error) {
	if !s.available() || !ValidID(id) {
		return Sync{}, ErrInvalid
	}
	sync, e := s.Repository.ReadSync(ctx, org, id)
	if e != nil {
		return sync, e
	}
	if sync.Terminal() {
		return sync, nil
	}
	return s.Repository.Stop(ctx, sync, "failed", "read_retry_exhausted")
}
func (s *Service) List(ctx context.Context, scope Scope, q Query) (Result, error) {
	if !s.available() || !q.Kind.Valid() || !Text(q.Keyword, 200) || q.Limit < 1 || q.Limit > 50 || len(q.After) > 4096 {
		return Result{}, ErrInvalid
	}
	if q.Kind == Products && q.Status != "" && q.Status != "active" && q.Status != "off" && q.Status != "unknown" {
		return Result{}, ErrInvalid
	}
	if q.Kind == Orders && q.Status != "" && q.Status != "transit" && q.Status != "exceptional" && q.Status != "unknown" {
		n, e := strconv.Atoi(q.Status)
		if e != nil || n < 1 || n > 9 {
			return Result{}, ErrInvalid
		}
	}
	stores, e := s.allowedStores(ctx, scope, q.Kind, false, q.Stores)
	if e != nil {
		return Result{}, e
	}
	q.Stores = stores
	q.Sources = map[string]string{}
	heads, latest, e := s.Repository.Heads(ctx, scope.OrganizationID, q.Kind, stores)
	if e != nil {
		return Result{}, e
	}
	if q.SyncID != "" {
		if len(stores) != 1 || !ValidID(q.SyncID) {
			return Result{}, ErrInvalid
		}
		chosen, e := s.Repository.ReadSync(ctx, scope.OrganizationID, q.SyncID)
		if e != nil || chosen.StoreID != stores[0] || chosen.Kind != q.Kind {
			return Result{}, ErrNotFound
		}
		heads = []Sync{chosen}
	}
	handles := []Merchant{}
	validated := map[Binding]Merchant{}
	selected := []Sync{}
	complete := len(stores) > 0
	for _, head := range heads {
		if head.Binding.ApplicationID == "" {
			complete = false
			continue
		}
		m, e := s.Access.Open(ctx, scope, head.StoreID, q.Kind, false, &head.Binding)
		if e != nil {
			if errors.Is(e, ErrUnavailable) {
				return Result{}, e
			}
			complete = false
			continue
		}
		q.Sources[head.StoreID] = head.ID
		handles = append(handles, m)
		validated[head.Binding] = m
		selected = append(selected, head)
		if head.Status != "completed" || head.Progress.Incomplete {
			complete = false
		}
	}
	if len(selected) != len(stores) {
		complete = false
	}
	currentLatest := []Sync{}
	for _, sync := range latest {
		if sync.Binding.ApplicationID == "" {
			continue
		}
		if validated[sync.Binding] == nil {
			m, e := s.Access.Open(ctx, scope, sync.StoreID, q.Kind, false, &sync.Binding)
			if e != nil {
				if errors.Is(e, ErrUnavailable) {
					return Result{}, e
				}
				continue
			}
			validated[sync.Binding] = m
			handles = append(handles, m)
		}
		currentLatest = append(currentLatest, sync)
	}
	zone := time.FixedZone("UTC+8", 8*3600)
	now := s.now().In(zone)
	q.TodayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone).UTC()
	result, e := s.Repository.List(ctx, scope.OrganizationID, q)
	if e != nil {
		return Result{}, e
	}
	for _, m := range handles {
		if e = m.Check(ctx); e != nil {
			return Result{}, e
		}
	}
	// Recheck the currently authorized directory too; aggregates must never retain
	// records from a store revoked while the DB query was running.
	if _, e = s.allowedStores(ctx, scope, q.Kind, false, stores); e != nil {
		return Result{}, e
	}
	result.Syncs = selected
	result.Latest = currentLatest
	result.Complete = complete
	return result, nil
}
func (s *Service) Detail(ctx context.Context, scope Scope, store, syncID, id string, kind Kind) (Record, error) {
	if !s.available() || !scope.Valid() || !ValidID(store) || !ValidID(syncID) || !Identity(id) || !kind.Valid() {
		return Record{}, ErrInvalid
	}
	sync, e := s.Repository.ReadSync(ctx, scope.OrganizationID, syncID)
	if e != nil || sync.StoreID != store || sync.Kind != kind {
		return Record{}, ErrNotFound
	}
	merchant, e := s.Access.Open(ctx, scope, store, kind, false, &sync.Binding)
	if e != nil {
		return Record{}, e
	}
	record, e := s.Repository.Record(ctx, scope.OrganizationID, store, kind, syncID, id)
	if e != nil {
		return Record{}, e
	}
	if record.ID != id || record.StoreID != store || record.SyncID != syncID {
		return Record{}, ErrNotFound
	}
	if kind == Orders {
		detail, e := merchant.OrderDetails(ctx, []string{id})
		if e != nil {
			if errors.Is(e, ErrUnavailable) && record.Order != nil && record.Order.ID == id && record.Order.Site == "shein-us" {
				if e = merchant.Check(ctx); e != nil {
					return Record{}, e
				}
				record.Stale = true
				return record, nil
			}
			return Record{}, e
		}
		if len(detail) != 1 || detail[0].ID != id || detail[0].Site != "shein-us" {
			return Record{}, ErrNotFound
		}
		if record.Order != nil {
			detail[0].IssuedAt = record.Order.IssuedAt
		}
		record.Order = &detail[0]
		record.ObservedAt = s.now()
	}
	if e = merchant.Check(ctx); e != nil {
		return Record{}, e
	}
	return record, nil
}
func (s *Service) Logistics(ctx context.Context, scope Scope, store, sync, order, pkg string) ([]Track, error) {
	if !Identity(pkg) {
		return nil, ErrInvalid
	}
	record, e := s.Detail(ctx, scope, store, sync, order, Orders)
	if e != nil {
		return nil, e
	}
	if record.Stale {
		return nil, ErrUnavailable
	}
	found := false
	for _, p := range record.Order.Packages {
		if p.ID == pkg {
			found = true
		}
	}
	if !found {
		return nil, ErrNotFound
	}
	// Reopening checks the same source again without passing a user-supplied waybill.
	selected, e := s.Repository.ReadSync(ctx, scope.OrganizationID, sync)
	if e != nil {
		return nil, e
	}
	merchant, e := s.Access.Open(ctx, scope, store, Orders, false, &selected.Binding)
	if e != nil {
		return nil, e
	}
	tracks, e := merchant.Track(ctx, order, pkg)
	if e != nil {
		return nil, e
	}
	if e = merchant.Check(ctx); e != nil {
		return nil, e
	}
	return tracks, nil
}
