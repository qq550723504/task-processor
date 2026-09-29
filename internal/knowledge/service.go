package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo    Repository
	objects KnowledgeObjectStore
}

func NewService(repo Repository, objects KnowledgeObjectStore) (*Service, error) {
	if repo == nil || objects == nil {
		return nil, ErrUnavailable
	}
	return &Service{repo: repo, objects: objects}, nil
}
func ValidID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}
func validScope(s Scope) bool {
	return len(s.OrganizationID) > 0 && len(s.OrganizationID) <= 128 && s.OrganizationID == strings.TrimSpace(s.OrganizationID) && len(s.ActorID) > 0 && len(s.ActorID) <= 256 && s.ActorID == strings.TrimSpace(s.ActorID)
}

func (s *Service) Mutate(ctx context.Context, c Command) (Result, error) {
	if !validScope(c.Scope) || !ValidID(c.Key) {
		return Result{}, ErrInvalid
	}
	switch c.Kind {
	case "base_create":
	case "base_update", "base_disable":
		if !ValidID(c.BaseID) || c.Version < 1 {
			return Result{}, ErrInvalid
		}
	case "source_disable":
		if !ValidID(c.SourceID) || c.Version < 1 {
			return Result{}, ErrInvalid
		}
	default:
		return Result{}, ErrInvalid
	}
	if c.Kind == "base_create" || c.Kind == "base_update" {
		var err error
		c.Name, err = NormalizeName(c.Name)
		if err != nil {
			return Result{}, err
		}
	}
	c.Fingerprint = fingerprint(c.Scope.ActorID, c.Kind, c.BaseID, c.SourceID, c.Name, c.Version)
	return s.repo.Apply(ctx, c)
}
func fingerprint(fields ...any) string {
	data, _ := json.Marshal(fields)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func Digest(data []byte) string { digest := sha256.Sum256(data); return hex.EncodeToString(digest[:]) }

func (s *Service) Upload(ctx context.Context, c Command, filename string, data []byte) (Result, error) {
	if !validScope(c.Scope) || !ValidID(c.Key) || (c.Kind != "source_create" && c.Kind != "revision_create") {
		return Result{}, ErrInvalid
	}
	if c.Kind == "source_create" && !ValidID(c.BaseID) || c.Kind == "revision_create" && (!ValidID(c.SourceID) || c.Version < 1) {
		return Result{}, ErrInvalid
	}
	contentType, err := DetectDocument(filename, data)
	if err != nil {
		return Result{}, err
	}
	c.Name, err = NormalizeName(c.Name)
	if err != nil {
		return Result{}, err
	}
	c.Upload = &Revision{Filename: filename, ContentType: contentType, SizeBytes: int64(len(data)), SHA256: Digest(data)}
	c.Fingerprint = fingerprint(c.Scope.ActorID, c.Kind, c.BaseID, c.SourceID, c.Name, c.Version, filename, contentType, c.Upload.SizeBytes, c.Upload.SHA256)
	result, err := s.repo.Apply(ctx, c)
	if err != nil {
		return Result{}, err
	}
	revision, claimed, err := s.repo.ClaimUpload(ctx, c.Scope.OrganizationID, result.Revision.ID, uuid.NewString())
	if err != nil {
		return Result{}, err
	}
	if !claimed {
		return result, nil
	}
	object := revisionObject(revision)
	object.Data = data
	// Leave five seconds of the 30s lease for the final database transition.
	ioContext, cancelIO := context.WithTimeout(ctx, 25*time.Second)
	defer cancelIO()
	// An earlier writer may have lost its response. Inspect the original identity
	// before any resend; absence must be authoritative, never a transport guess.
	inspectContext, cancelInspect := context.WithTimeout(ioContext, 5*time.Second)
	inspection, inspectErr := s.objects.Inspect(inspectContext, object)
	cancelInspect()
	if inspectErr != nil {
		return result, nil
	}
	if inspection.Exists {
		if !matches(object, inspection) {
			if finishErr := s.repo.Finish(ctx, revision, ParseResult{Failure: "OBJECT_INTEGRITY_FAILURE"}); finishErr != nil {
				return Result{}, finishErr
			}
			return Result{}, ErrIntegrity
		}
		if err := s.repo.ConfirmObject(ctx, revision); err != nil {
			return Result{}, err
		}
		return s.uploadResult(ctx, c, result, revision)
	}
	putContext, cancel := context.WithTimeout(ioContext, 20*time.Second)
	err = s.objects.PutImmutable(putContext, object)
	cancel()
	if err != nil {
		inspectContext, cancelInspect := context.WithTimeout(ioContext, 5*time.Second)
		inspection, inspectErr := s.objects.Inspect(inspectContext, object)
		cancelInspect()
		if inspectErr != nil || !inspection.Exists {
			return result, nil
		} // durable ADMITTED; recovery keeps exact identity
		if !matches(object, inspection) {
			if finishErr := s.repo.Finish(ctx, revision, ParseResult{Failure: "OBJECT_INTEGRITY_FAILURE"}); finishErr != nil {
				return Result{}, finishErr
			}
			return Result{}, ErrIntegrity
		}
	}
	if err := s.repo.ConfirmObject(ctx, revision); err != nil && !errors.Is(err, ErrLeaseLost) {
		return Result{}, err
	}
	return s.uploadResult(ctx, c, result, revision)
}
func (s *Service) uploadResult(ctx context.Context, c Command, result Result, revision Revision) (Result, error) {
	source, err := s.repo.GetSource(ctx, c.Scope.OrganizationID, result.Source.ID)
	if err != nil {
		return Result{}, err
	}
	result.Source = &source
	if source.LatestRevision != nil && source.LatestRevision.ID == revision.ID {
		result.Revision = source.LatestRevision
	}
	return result, nil
}
func revisionObject(r Revision) Object {
	return Object{Key: r.ObjectKey, SHA256: r.SHA256, ContentType: r.ContentType, SizeBytes: r.SizeBytes}
}
func matches(o Object, i Inspection) bool {
	return i.Exists && o.SHA256 == i.SHA256 && o.SizeBytes == i.SizeBytes
}
func (s *Service) ListBases(ctx context.Context, scope Scope, page, size int) ([]Base, int64, error) {
	if !validScope(scope) || page < 1 || page > 100000 || size < 1 || size > 100 {
		return nil, 0, ErrInvalid
	}
	return s.repo.ListBases(ctx, scope.OrganizationID, page, size)
}
func (s *Service) GetBase(ctx context.Context, scope Scope, id string) (Base, error) {
	if !validScope(scope) || !ValidID(id) {
		return Base{}, ErrInvalid
	}
	return s.repo.GetBase(ctx, scope.OrganizationID, id)
}
func (s *Service) ListSources(ctx context.Context, scope Scope, id string) ([]Source, error) {
	if !validScope(scope) || !ValidID(id) {
		return nil, ErrInvalid
	}
	return s.repo.ListSources(ctx, scope.OrganizationID, id)
}
func (s *Service) GetSource(ctx context.Context, scope Scope, id string) (Source, error) {
	if !validScope(scope) || !ValidID(id) {
		return Source{}, ErrInvalid
	}
	return s.repo.GetSource(ctx, scope.OrganizationID, id)
}
func (s *Service) Preview(ctx context.Context, scope Scope, source, revision string) (Preview, error) {
	if !validScope(scope) || !ValidID(source) || !ValidID(revision) {
		return Preview{}, ErrInvalid
	}
	return s.repo.Preview(ctx, scope.OrganizationID, source, revision)
}
