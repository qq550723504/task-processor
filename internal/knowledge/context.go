package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"task-processor/internal/agent"
)

const (
	ContextKind             = "knowledge"
	ContextPolicyVersion    = "knowledge-context-v1"
	MaxContextRevisionBytes = 16 << 10
	MaxContextTextBytes     = 48 << 10
	MaxContextCitations     = 64
	MaxContextPayloadBytes  = 96 << 10
	DispatchPermitLease     = 5*time.Minute + 30*time.Second
)

var ErrContextTooLarge = errors.New("KNOWLEDGE_CONTEXT_TOO_LARGE")
var ErrForbidden = errors.New("KNOWLEDGE_FORBIDDEN")
var ErrSelectionChanged = errors.New("KNOWLEDGE_SELECTION_CHANGED")
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ContextRequest is a server-constructed pre-Start command. Resolved sources,
// revisions and digests deliberately have no fields in this command.
type ContextRequest struct {
	Scope                         Scope
	Binding                       agent.Binding
	Key, Selection, PolicyVersion string
	ExpectedRevisionSetDigest     string
}
type SelectionRevisionSetRef struct {
	BaseID string
	Digest string
}
type ContextSnapshotRef struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (r ContextSnapshotRef) Valid() bool {
	return r.Kind == ContextKind && ValidID(r.ID) && digestPattern.MatchString(r.Digest)
}

type Citation struct {
	ID            string `json:"id"`
	BaseID        string `json:"knowledgeBaseId"`
	SourceID      string `json:"sourceId"`
	RevisionID    string `json:"revisionId"`
	ContentDigest string `json:"contentDigest"`
	Location      string `json:"location"`
}
type ContextEntry struct {
	SourceID      string          `json:"sourceId"`
	RevisionID    string          `json:"revisionId"`
	Name          string          `json:"name"`
	State         ProcessingState `json:"state"`
	Warning       string          `json:"warning,omitempty"`
	Text          string          `json:"text"`
	ContentDigest string          `json:"contentDigest"`
	Citation      Citation        `json:"citation"`
}

// ContextBundle is protected, bounded Knowledge content. It must never be
// stored in an Agent checkpoint, browser/session or ordinary log.
type ContextBundle struct {
	BaseID  string         `json:"knowledgeBaseId"`
	Binding agent.Binding  `json:"binding"`
	Entries []ContextEntry `json:"entries"`
}
type CitationContent struct {
	Citation            Citation
	Name, Text, Warning string
	State               ProcessingState
}
type DispatchPermit struct {
	ID, OrganizationID, ActorID, InvocationID string
	Ref                                       ContextSnapshotRef
	ExpiresAt                                 time.Time
}

// KnowledgeAuthorizer resolves verified identity, live Effective Organization
// grants and knowledge.read for every operation; a cached Scope is insufficient.
type KnowledgeAuthorizer interface {
	AuthorizeKnowledge(context.Context) (Scope, error)
}
type KnowledgeContextMaterializer interface {
	Materialize(context.Context, ContextRequest) (ContextSnapshotRef, error)
}
type KnowledgeSelectionObserver interface {
	ObserveSelection(context.Context, Scope, string) (SelectionRevisionSetRef, error)
}
type KnowledgeContextReader interface {
	ReadContext(context.Context, Scope, ContextSnapshotRef) (ContextBundle, error)
}
type KnowledgeCitationReader interface {
	ReadCitation(context.Context, Scope, ContextSnapshotRef, string) (CitationContent, error)
}
type KnowledgeDispatchPermitGate interface {
	AcquireDispatchPermit(context.Context, Scope, ContextSnapshotRef, string) (DispatchPermit, error)
	ReleaseDispatchPermit(context.Context, DispatchPermit) error
}
type DispatchPermitRecovery interface{ RecoverExpiredDispatchPermits(context.Context) error }
type ContextRepository interface {
	Materialize(context.Context, ContextRequest) (ContextSnapshotRef, error)
	ValidateMaterializedRequest(context.Context, ContextRequest, ContextSnapshotRef) error
	ReadContext(context.Context, Scope, ContextSnapshotRef) (ContextBundle, error)
	AcquireDispatchPermit(context.Context, Scope, ContextSnapshotRef, string) (DispatchPermit, error)
	ReleaseDispatchPermit(context.Context, DispatchPermit) error
	DispatchPermitRecovery
}

func MaterializationFingerprint(r ContextRequest) (string, error) {
	if !validScope(r.Scope) || !r.Binding.Valid() || !agent.ValidID(r.Key) || !agent.ValidID(r.PolicyVersion) || len(r.Selection) != len("knowledge-base:")+36 || !ValidID(r.Selection[len("knowledge-base:"):]) || r.Selection[:len("knowledge-base:")] != "knowledge-base:" ||
		(r.ExpectedRevisionSetDigest != "" && !digestPattern.MatchString(r.ExpectedRevisionSetDigest)) {
		return "", ErrInvalid
	}
	if r.ExpectedRevisionSetDigest != "" {
		return fingerprint(r.Binding, r.Selection, r.PolicyVersion, r.ExpectedRevisionSetDigest), nil
	}
	return fingerprint(r.Binding, r.Selection, r.PolicyVersion), nil
}

// EncodeContextBundle enforces hard bounds before persistence. No prefix,
// truncation or omitted source is a successful materialization.
func EncodeContextBundle(b ContextBundle) ([]byte, error) {
	if len(b.Entries) > MaxActiveSources || len(b.Entries) > MaxContextCitations {
		return nil, ErrContextTooLarge
	}
	if len(b.Entries) == 0 {
		return nil, ErrNotReadable
	}
	total := 0
	for _, e := range b.Entries {
		if len(e.Text) > MaxContextRevisionBytes {
			return nil, ErrContextTooLarge
		}
		total += len(e.Text)
	}
	if total > MaxContextTextBytes {
		return nil, ErrContextTooLarge
	}
	data, err := json.Marshal(b)
	if err != nil {
		return nil, ErrIntegrity
	}
	if len(data) > MaxContextPayloadBytes {
		return nil, ErrContextTooLarge
	}
	return data, nil
}

type ContextService struct {
	repo ContextRepository
	auth KnowledgeAuthorizer
}

// Context borrows the existing Knowledge owner's repository/pool. Future
// consumers supply their request-local live authorizer; no schema or new pool
// is opened, and no public retrieval route is registered.
func (s *Service) Context(auth KnowledgeAuthorizer) (*ContextService, error) {
	repo, ok := s.repo.(ContextRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	return NewContextService(repo, auth)
}

func NewContextService(repo ContextRepository, auth KnowledgeAuthorizer) (*ContextService, error) {
	if repo == nil || auth == nil {
		return nil, ErrUnavailable
	}
	return &ContextService{repo: repo, auth: auth}, nil
}
func (s *ContextService) admit(ctx context.Context, expected Scope) error {
	if !validScope(expected) {
		return ErrInvalid
	}
	actual, err := s.auth.AuthorizeKnowledge(ctx)
	if err != nil || actual != expected {
		return ErrForbidden
	}
	return ctx.Err()
}
func (s *ContextService) Materialize(ctx context.Context, r ContextRequest) (ContextSnapshotRef, error) {
	if _, err := MaterializationFingerprint(r); err != nil {
		return ContextSnapshotRef{}, err
	}
	if err := s.admit(ctx, r.Scope); err != nil {
		return ContextSnapshotRef{}, err
	}
	return s.repo.Materialize(ctx, r)
}

func (s *ContextService) ObserveSelection(ctx context.Context, scope Scope, baseID string) (SelectionRevisionSetRef, error) {
	if s == nil || !ValidID(baseID) {
		return SelectionRevisionSetRef{}, ErrInvalid
	}
	if err := s.admit(ctx, scope); err != nil {
		return SelectionRevisionSetRef{}, err
	}
	observer, ok := s.repo.(KnowledgeSelectionObserver)
	if !ok {
		return SelectionRevisionSetRef{}, ErrUnavailable
	}
	return observer.ObserveSelection(ctx, scope, baseID)
}

// ValidateMaterializedRequest validates only immutable command metadata for an
// already claimed consumer operation. It authorizes no content read, new bundle,
// resume or dispatch; the consumer must compare the complete saved request.
func (s *ContextService) ValidateMaterializedRequest(ctx context.Context, r ContextRequest, ref ContextSnapshotRef) error {
	if _, err := MaterializationFingerprint(r); err != nil || !ref.Valid() {
		return ErrInvalid
	}
	if err := s.admit(ctx, r.Scope); err != nil {
		return err
	}
	return s.repo.ValidateMaterializedRequest(ctx, r, ref)
}
func (s *ContextService) ReadContext(ctx context.Context, scope Scope, ref ContextSnapshotRef) (ContextBundle, error) {
	if !ref.Valid() {
		return ContextBundle{}, ErrInvalid
	}
	if err := s.admit(ctx, scope); err != nil {
		return ContextBundle{}, err
	}
	return s.repo.ReadContext(ctx, scope, ref)
}
func (s *ContextService) ReadCitation(ctx context.Context, scope Scope, ref ContextSnapshotRef, id string) (CitationContent, error) {
	if !ValidID(id) {
		return CitationContent{}, ErrInvalid
	}
	b, err := s.ReadContext(ctx, scope, ref)
	if err != nil {
		return CitationContent{}, err
	}
	for _, e := range b.Entries {
		if e.Citation.ID == id {
			return CitationContent{Citation: e.Citation, Name: e.Name, Text: e.Text, Warning: e.Warning, State: e.State}, nil
		}
	}
	return CitationContent{}, ErrNotFound
}
func (s *ContextService) AcquireDispatchPermit(ctx context.Context, scope Scope, ref ContextSnapshotRef, invocation string) (DispatchPermit, error) {
	if !ref.Valid() || !agent.ValidID(invocation) {
		return DispatchPermit{}, ErrInvalid
	}
	if err := s.admit(ctx, scope); err != nil {
		return DispatchPermit{}, err
	}
	return s.repo.AcquireDispatchPermit(ctx, scope, ref, invocation)
}

// Release is capability-bound cleanup, including after permission revocation.
// It authorizes no content release, AI outcome write or model redispatch.
func (s *ContextService) ReleaseDispatchPermit(ctx context.Context, permit DispatchPermit) error {
	return s.repo.ReleaseDispatchPermit(ctx, permit)
}
