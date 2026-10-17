package skill

import (
	"context"
	"task-processor/internal/agent"
)

type Service struct {
	repo    Repository
	resolve ResolvePrincipal
}

func NewService(repo Repository, resolve ResolvePrincipal) (*Service, error) {
	if repo == nil || resolve == nil {
		return nil, ErrUnavailable
	}
	return &Service{repo: repo, resolve: resolve}, nil
}
func (s *Service) Principal(ctx context.Context) (Principal, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return Principal{}, ErrUnavailable
	}
	p, e := s.resolve(ctx)
	if e != nil {
		return p, e
	}
	if !agent.ValidID(p.UserID) {
		return p, ErrForbidden
	}
	return p, nil
}
func OwnerFor(p Principal, space string) (Owner, error) {
	switch space {
	case "personal":
		return Owner{Kind: UserOwner, ID: p.UserID}, nil
	case "organization":
		if !agent.ValidID(p.OrganizationID) {
			return Owner{}, ErrForbidden
		}
		return Owner{Kind: OrganizationOwner, ID: p.OrganizationID}, nil
	case "official":
		return Owner{Kind: PlatformOwner, ID: PlatformID}, nil
	default:
		return Owner{}, ErrInvalid
	}
}
func (s *Service) List(ctx context.Context, space, cursor string, size int) ([]Version, string, Principal, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return nil, "", p, e
	}
	o, e := OwnerFor(p, space)
	if e != nil {
		return nil, "", p, e
	}
	items, next, e := s.repo.List(ctx, p, o, cursor, size)
	return items, next, p, e
}
func (s *Service) Read(ctx context.Context, space, id string, version uint64) (Version, Principal, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return Version{}, p, e
	}
	o, e := OwnerFor(p, space)
	if e != nil {
		return Version{}, p, e
	}
	v, e := s.repo.Read(ctx, p, o, id, version)
	return v, p, e
}
func (s *Service) Execute(ctx context.Context, space string, c Command) (Receipt, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return Receipt{}, e
	}
	o, e := OwnerFor(p, space)
	if e != nil {
		return Receipt{}, e
	}
	c.Principal, c.Owner = p, o
	return s.repo.Execute(ctx, c, s.resolve)
}
func (s *Service) Export(ctx context.Context, space, id string, version uint64) (Package, error) {
	v, p, e := s.Read(ctx, space, id, version)
	if e != nil {
		return Package{}, e
	}
	if !Authorized(p, v, Export) {
		return Package{}, ErrForbidden
	}
	return v.Package, nil
}
func (s *Service) Share(ctx context.Context, key, id string, version uint64, digest string, consent bool) (ShareRequest, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return ShareRequest{}, e
	}
	v, e := s.repo.Read(ctx, p, Owner{Kind: UserOwner, ID: p.UserID}, id, version)
	if e != nil {
		return ShareRequest{}, e
	}
	if v.Package.Digest != digest {
		return ShareRequest{}, ErrConflict
	}
	return s.repo.Share(ctx, p, key, id, version, consent, s.resolve)
}
func (s *Service) Shares(ctx context.Context, organization bool) ([]ShareRequest, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return nil, e
	}
	return s.repo.Shares(ctx, p, organization)
}
func (s *Service) ReadShared(ctx context.Context, id string) (Version, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return Version{}, e
	}
	return s.repo.ReadShared(ctx, p, id)
}
func (s *Service) DecideShare(ctx context.Context, key, id, action string) (ShareRequest, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return ShareRequest{}, e
	}
	return s.repo.DecideShare(ctx, p, key, id, action, s.resolve)
}

type VersionLister interface {
	Versions(context.Context, Principal, Owner, string) ([]Version, error)
}

func (s *Service) Versions(ctx context.Context, space, id string) ([]Version, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return nil, e
	}
	o, e := OwnerFor(p, space)
	if e != nil {
		return nil, e
	}
	repo, ok := s.repo.(VersionLister)
	if !ok {
		return nil, ErrUnavailable
	}
	return repo.Versions(ctx, p, o, id)
}

type CandidateLister interface {
	Candidates(context.Context, Principal) ([]Version, error)
}

func (s *Service) InstallOfficial(ctx context.Context) (OfficialInstallResult, error) {
	p, err := s.Principal(ctx)
	if err != nil {
		return OfficialInstallResult{}, err
	}
	if !p.PlatformMaintainer {
		return OfficialInstallResult{}, ErrForbidden
	}
	repo, ok := s.repo.(OfficialInstaller)
	if !ok {
		return OfficialInstallResult{}, ErrUnavailable
	}
	return repo.InstallOfficial(ctx, p, s.resolve)
}

func (s *Service) Candidates(ctx context.Context) ([]Version, error) {
	p, e := s.Principal(ctx)
	if e != nil {
		return nil, e
	}
	repo, ok := s.repo.(CandidateLister)
	if !ok {
		return nil, ErrUnavailable
	}
	return repo.Candidates(ctx, p)
}

type TargetCandidateLister interface {
	CandidatesForTarget(context.Context, Principal, string) ([]Version, error)
}

func (s *Service) CandidatesForTarget(ctx context.Context, target string) ([]Version, error) {
	if target != TitleTarget && target != ImageTarget {
		return nil, ErrInvalid
	}
	p, err := s.Principal(ctx)
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(TargetCandidateLister)
	if !ok {
		return nil, ErrUnavailable
	}
	return repo.CandidatesForTarget(ctx, p, target)
}
