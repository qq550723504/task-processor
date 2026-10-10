package reportcenter

import "context"

func (s *Service) authorize(ctx context.Context, scope Scope, manage bool) (context.Context, error) {
	if !scope.Valid() || s == nil || s.Repository == nil || s.Authorize == nil {
		return nil, ErrForbidden
	}
	return s.Authorize(ctx, scope, manage)
}
func (s *Service) Source(ctx context.Context, scope Scope, kind, id string) (SourceInfo, error) {
	ctx, e := s.authorize(ctx, scope, false)
	if e != nil {
		return SourceInfo{}, e
	}
	if !KindValid(kind) || !UUID(id) {
		return SourceInfo{}, ErrInvalid
	}
	if s.Sources == nil {
		return SourceInfo{}, ErrUnavailable
	}
	snap, e := s.Sources.Read(ctx, scope, kind, id)
	if e != nil {
		return SourceInfo{}, e
	}
	if snap.Ref.Kind != kind || snap.Ref.ID != id {
		return SourceInfo{}, ErrUnavailable
	}
	if _, _, e = snap.Bytes(); e != nil {
		return SourceInfo{}, e
	}
	return snap.SourceInfo, nil
}
func (s *Service) Save(ctx context.Context, scope Scope, key string, ref SourceRef) (Result, error) {
	ctx, e := s.authorize(ctx, scope, true)
	if e != nil {
		return Result{}, e
	}
	if !UUID(key) || !ref.Valid() {
		return Result{}, ErrInvalid
	}
	fp := Fingerprint("save", ref)
	id, e := s.Repository.Lookup(ctx, scope, key, fp)
	if e != nil {
		return Result{}, e
	}
	if id != "" {
		r, e := s.Repository.Read(ctx, scope, id)
		return Result{CommandID: key, Report: r, Replayed: true}, e
	}
	if s.Sources == nil {
		return Result{}, ErrUnavailable
	}
	snap, e := s.Sources.Read(ctx, scope, ref.Kind, ref.ID)
	if e != nil {
		return Result{}, e
	}
	if snap.Ref != ref {
		return Result{}, ErrConflict
	}
	if _, _, e = snap.Bytes(); e != nil {
		return Result{}, e
	}
	result, e := s.Repository.Save(ctx, scope, key, fp, snap, func(c context.Context) error { _, e := s.authorize(c, scope, true); return e })
	result.CommandID = key
	return result, e
}
func (s *Service) Favorite(ctx context.Context, scope Scope, key, id string, target bool) (Result, error) {
	ctx, e := s.authorize(ctx, scope, true)
	if e != nil {
		return Result{}, e
	}
	if !UUID(key) || !UUID(id) {
		return Result{}, ErrInvalid
	}
	fp := Fingerprint("favorite", struct {
		ID     string
		Target bool
	}{id, target})
	result, e := s.Repository.Favorite(ctx, scope, key, fp, id, target, func(c context.Context) error { _, e := s.authorize(c, scope, true); return e })
	result.CommandID = key
	return result, e
}
func (s *Service) Read(ctx context.Context, scope Scope, id string) (Report, error) {
	ctx, e := s.authorize(ctx, scope, false)
	if e != nil {
		return Report{}, e
	}
	if !UUID(id) {
		return Report{}, ErrInvalid
	}
	return s.Repository.Read(ctx, scope, id)
}
func (s *Service) List(ctx context.Context, scope Scope, f Filter) (Page, error) {
	ctx, e := s.authorize(ctx, scope, false)
	if e != nil {
		return Page{}, e
	}
	if !f.Valid() {
		return Page{}, ErrInvalid
	}
	return s.Repository.List(ctx, scope, f)
}
func (s *Service) Summary(ctx context.Context, scope Scope) (Summary, error) {
	ctx, e := s.authorize(ctx, scope, false)
	if e != nil {
		return Summary{}, e
	}
	return s.Repository.Summary(ctx, scope)
}
