package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	reader       catalog.VersionedSnapshotReader
	sourceReader SourcePublicationGateway
	store        Store
	proposer     enrichment.Proposer
	auth         Authorizer
}

func NewService(reader catalog.VersionedSnapshotReader, sourceReader SourcePublicationGateway, store Store, proposer enrichment.Proposer, auth Authorizer) (*Service, error) {
	if reader == nil || sourceReader == nil || store == nil || proposer == nil || auth == nil {
		return nil, ErrUnavailable
	}
	return &Service{reader: reader, sourceReader: sourceReader, store: store, proposer: proposer, auth: auth}, nil
}
func (s *Service) authorize(ctx context.Context, write, admin bool) (Scope, error) {
	if err := ctx.Err(); err != nil {
		return Scope{}, err
	}
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !ValidKey(i.UserID) || !ValidKey(i.EffectiveOrganizationID) || i.TenantID != i.EffectiveOrganizationID || !time.Now().Before(i.TokenExpiresAt) {
		return Scope{}, ErrForbidden
	}
	// Scope roles are produced by the existing organization resolver. Configured
	// global user grants cannot substitute for a grant in the effective org.
	a := Scope{i.EffectiveOrganizationID, i.UserID, s.auth.IsTenantAdmin("", i.Roles)}
	if !s.auth.Authorize("", i.Roles, authz.PermissionListingKitAdminRead) || write && !s.auth.Authorize("", i.Roles, authz.PermissionListingKitAdminWrite) || admin && !a.Admin {
		return Scope{}, ErrForbidden
	}
	return a, nil
}
func operation(a Scope, key, kind, id string, input any) (Operation, error) {
	if !ValidKey(key) {
		return Operation{}, ErrInvalid
	}
	raw, err := json.Marshal([]any{kind, id, input})
	if err != nil {
		return Operation{}, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	return Operation{a, key, hex.EncodeToString(sum[:])}, nil
}
func (s *Service) Create(ctx context.Context, key string, in CreateInput) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	a, err := s.authorize(ctx, true, false)
	if err != nil {
		return View{}, err
	}
	if !ValidKey(in.ProductKey) || in.BaseVersion == 0 || in.BaseVersion > 1<<63-1 {
		return View{}, ErrInvalid
	}
	op, err := operation(a, key, "create", "", in)
	if err != nil {
		return View{}, err
	}
	if v, found, e := s.store.Preflight(ctx, op); e != nil || found {
		return v, e
	}
	base, source, err := s.source(ctx, s.reader, s.sourceReader, a.Org, in)
	if err != nil {
		return View{}, err
	}
	policy := enrichment.PolicySnapshot{Version: "title-review-v1", AllowedFields: []string{"title"}, RequiredFields: []string{"title"}}
	proposal, err := s.proposer.Propose(ctx, enrichment.Request{Snapshot: base.Snapshot, Source: source, Policy: policy})
	if err != nil {
		return View{}, err
	}
	if len(proposal.Changes) != 1 || proposal.Changes[0].Field != "title" || !proposal.Validation.Valid {
		return View{}, ErrInvalid
	}
	if err = ValidateTitle(proposal.Changes[0].Value); err != nil {
		return View{}, err
	}
	ctx, err = s.sourceReader.AuthorizeRead(ctx)
	if err != nil {
		return View{}, mapSourceReadError(err)
	}
	r := Record{ID: uuid.NewString(), Org: a.Org, Owner: a.Actor, Input: in, BasePublicationID: base.PublicationID, Policy: policy.Version, Before: base.Snapshot.Title, Title: proposal.Changes[0].Value, State: "pending", Revision: 1, Original: proposal}
	return s.store.Run(ctx, op, func(tx Tx) (View, error) {
		if v, found, e := tx.Replay(); e != nil || found {
			return v, e
		}
		rechecked, _, e := s.source(ctx, tx.Reader(), tx.SourceReader(), a.Org, in)
		if e != nil {
			return View{}, e
		}
		if rechecked.Identity != base.Identity || rechecked.Version != base.Version ||
			rechecked.PublicationID != base.PublicationID || !reflect.DeepEqual(rechecked.Snapshot, base.Snapshot) {
			return View{}, ErrConflict
		}
		if e := tx.Save(r); e != nil {
			return View{}, e
		}
		v := r.View()
		return v, tx.Complete(v)
	})
}
func validID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id && parsed != uuid.Nil
}
func (s *Service) Get(ctx context.Context, id string) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	a, err := s.authorize(ctx, false, false)
	if err != nil {
		return View{}, err
	}
	if !validID(id) {
		return View{}, ErrInvalid
	}
	r, err := s.store.Read(ctx, a, id)
	if err != nil {
		return View{}, err
	}
	return r.View(), nil
}
func (s *Service) Decide(ctx context.Context, key, id string, in DecisionInput) (View, error) {
	return s.change(ctx, key, id, "decision", in, in.Action != "edit", in.Action != "reject", func(ctx context.Context, tx Tx, r *Record, a Scope) error {
		if err := r.Decide(a.Actor, in); err != nil {
			return err
		}
		if in.Action != "reject" {
			_, err := s.validatePatch(ctx, tx, r, a)
			return err
		}
		return nil
	})
}
func (s *Service) validatePatch(ctx context.Context, tx Tx, r *Record, a Scope) (catalog.PublishedSnapshot, error) {
	if err := ValidateTitle(r.Title); err != nil {
		return catalog.PublishedSnapshot{}, err
	}
	base, source, err := s.source(ctx, tx.Reader(), tx.SourceReader(), a.Org, r.Input)
	if err != nil {
		return base, err
	}
	evidence, err := enrichment.CanonicalEvidenceID(source)
	if err != nil {
		return base, err
	}
	if base.PublicationID != r.BasePublicationID || base.Snapshot.Title != r.Before || r.Policy != "title-review-v1" || len(r.Original.Changes) != 1 || r.Original.Changes[0].Field != "title" || len(r.Original.Changes[0].EvidenceIDs) != 1 || r.Original.Changes[0].EvidenceIDs[0] != evidence {
		return base, ErrConflict
	}
	return base, nil
}
func (s *Service) Apply(ctx context.Context, key, id string, in ApplyInput) (View, error) {
	return s.change(ctx, key, id, "apply", in, true, true, func(ctx context.Context, tx Tx, r *Record, a Scope) error {
		if r.State != "accepted" || in.ExpectedRevision != r.Revision || in.ExpectedRevision == 0 {
			return ErrConflict
		}
		base, err := s.validatePatch(ctx, tx, r, a)
		if err != nil {
			return err
		}
		snapshot, err := catalog.CloneProductSnapshot(base.Snapshot)
		if err != nil {
			return err
		}
		snapshot.Title = r.Title
		raw, _ := json.Marshal([]any{r.Org, r.ID, r.Revision})
		sum := sha256.Sum256(raw)
		published, err := tx.Publisher().Publish(ctx, catalog.PublishRequest{Identity: base.Identity, ExpectedBaseVersion: &base.Version, PublicationID: "review:" + hex.EncodeToString(sum[:]), Snapshot: snapshot})
		if err != nil {
			return err
		}
		r.State = "applied"
		r.Receipt = &Receipt{r.ID, r.Revision, published.Version, published.PublicationID, a.Actor, time.Now().UTC()}
		return nil
	})
}
func (s *Service) change(ctx context.Context, key, id, kind string, input any, admin, sourceRead bool, mutate func(context.Context, Tx, *Record, Scope) error) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	a, err := s.authorize(ctx, true, admin)
	if err != nil {
		return View{}, err
	}
	if !validID(id) {
		return View{}, ErrInvalid
	}
	op, err := operation(a, key, kind, id, input)
	if err != nil {
		return View{}, err
	}
	if v, found, e := s.store.Preflight(ctx, op); e != nil || found {
		return v, e
	}
	if sourceRead {
		if s.sourceReader == nil {
			return View{}, ErrUnavailable
		}
		ctx, err = s.sourceReader.AuthorizeRead(ctx)
		if err != nil {
			return View{}, mapSourceReadError(err)
		}
	}
	return s.store.Run(ctx, op, func(tx Tx) (View, error) {
		if v, found, e := tx.Replay(); e != nil || found {
			return v, e
		}
		r, e := tx.Load(id)
		if e != nil {
			return View{}, e
		}
		if e = mutate(ctx, tx, &r, a); e != nil {
			return View{}, e
		}
		if e = tx.Save(r); e != nil {
			return View{}, e
		}
		v := r.View()
		return v, tx.Complete(v)
	})
}
