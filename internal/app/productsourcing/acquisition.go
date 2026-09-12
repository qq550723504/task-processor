package productsourcing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type AcquisitionService struct {
	operations sourcing.AcquisitionOperationStore
	provider   sourcing.PublicAcquirer
	publisher  sourcing.AcquisitionPublisher
	reader     catalog.CompleteSnapshotReader
	authorizer sourcing.PublicationAuthorizer
}

func NewAcquisitionService(store sourcing.AcquisitionOperationStore, provider sourcing.PublicAcquirer, publisher sourcing.AcquisitionPublisher, reader catalog.CompleteSnapshotReader, authorizer sourcing.PublicationAuthorizer) (*AcquisitionService, error) {
	if store == nil || publisher == nil || authorizer == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	return &AcquisitionService{store, provider, publisher, reader, authorizer}, nil
}

func (s *AcquisitionService) Acquire(ctx context.Context, key, source string) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	request, err := s.request(ctx, key, source)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	op, fetchClaim, err := s.operations.Start(ctx, request)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	replayed := !fetchClaim
	if op.State == sourcing.AcquisitionAcquiring {
		if !fetchClaim {
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
		}
		if s.provider == nil || s.reader == nil {
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
		}
		if err := s.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		evidence, err := s.provider.Acquire(ctx, op.Source)
		if err != nil {
			return sourcing.AcquisitionResult{}, s.failFetch(ctx, op, err)
		}
		envelope, err := sourcing.MapAcquisitionEvidence(op.Source, evidence, "public_http", op.ID)
		if err != nil {
			return sourcing.AcquisitionResult{}, s.failFetch(ctx, op, err)
		}
		if err := s.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
		if err != nil {
			return sourcing.AcquisitionResult{}, s.failFetch(ctx, op, err)
		}
		base := uint64(0)
		current, err := s.reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: op.Scope.OrganizationID, ProductKey: productKey})
		if err == nil {
			base = current.Version
		} else if !errors.Is(err, catalog.ErrSnapshotNotReady) {
			return sourcing.AcquisitionResult{}, err
		}
		command := sourcing.PublicationCommand{PublicationID: publicationID, ProductKey: productKey, Producer: sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"}, ExpectedBaseVersion: &base, Envelope: envelope}
		op, err = s.operations.Prepare(ctx, op, command)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
	}
	if op.State == sourcing.AcquisitionFailed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionFailed
	}
	publishClaim := false
	if op.State == sourcing.AcquisitionPrepared {
		op, publishClaim, err = s.operations.Claim(ctx, op)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
	}
	return s.resolve(ctx, op, publishClaim, replayed)
}

func (s *AcquisitionService) Verify(ctx context.Context, key, source string) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	request, err := s.request(ctx, key, source)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	op, err := s.operations.ByKey(ctx, request.Scope, key)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	return s.resolve(ctx, op, false, true)
}

func (s *AcquisitionService) Read(ctx context.Context, operationID string) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	scope, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if !canonicalAcquisitionKey(operationID) {
		return sourcing.AcquisitionResult{}, sourcing.ErrInvalidAcquisition
	}
	op, err := s.operations.ByID(ctx, scope, operationID)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if op.Scope != scope || op.ID != operationID {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	result := sourcing.AcquisitionResult{Operation: op}
	if op.State == sourcing.AcquisitionPublished {
		if op.Command == nil {
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
		}
		persisted, err := s.exact(ctx, op)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		result.Publication = &persisted
	}
	return result, nil
}

func acquisitionOperation(scope sourcing.PublicationScope, key string, source sourcing.AcquisitionSource) sourcing.AcquisitionOperation {
	identity, _ := json.Marshal([]string{scope.OrganizationID, scope.ActorID, key})
	id := uuid.NewSHA1(uuid.NameSpaceURL, identity).String()
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", source.URL})
	hash := sha256.Sum256(input)
	return sourcing.AcquisitionOperation{Scope: scope, Key: key, ID: id, Source: source, Fingerprint: hex.EncodeToString(hash[:])}
}

func canonicalAcquisitionKey(key string) bool {
	parsed, err := uuid.Parse(key)
	return err == nil && parsed != uuid.Nil && parsed.String() == key && parsed.Variant() == uuid.RFC4122
}

func (s *AcquisitionService) request(ctx context.Context, key, input string) (sourcing.AcquisitionOperation, error) {
	scope, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return sourcing.AcquisitionOperation{}, err
	}
	if !canonicalAcquisitionKey(key) {
		return sourcing.AcquisitionOperation{}, sourcing.ErrInvalidAcquisition
	}
	source, err := sourcing.Canonical1688Source(input)
	if err != nil {
		return sourcing.AcquisitionOperation{}, err
	}
	return acquisitionOperation(scope, key, source), nil
}

func sameAcquisition(request, stored sourcing.AcquisitionOperation) error {
	if request.Scope != stored.Scope || request.ID != stored.ID || request.Key != stored.Key {
		return sourcing.ErrAcquisitionUnavailable
	}
	if request.Fingerprint != stored.Fingerprint || request.Source != stored.Source {
		return sourcing.ErrAcquisitionConflict
	}
	return nil
}

func (s *AcquisitionService) authorizeScope(ctx context.Context, expected sourcing.PublicationScope) error {
	scope, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return err
	}
	if scope != expected {
		return sourcing.ErrPublicationForbidden
	}
	return nil
}

func (s *AcquisitionService) failFetch(ctx context.Context, op sourcing.AcquisitionOperation, cause error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	code := "SOURCE_UNAVAILABLE"
	if errors.Is(cause, sourcing.ErrInvalidAcquisition) {
		code = "INVALID_SOURCE"
	}
	if errors.Is(cause, sourcing.ErrSourcePublicationTooLarge) {
		code = "SOURCE_TOO_LARGE"
	}
	if err := s.operations.Finish(ctx, op, sourcing.AcquisitionFailed, code); err != nil {
		return err
	}
	return sourcing.ErrAcquisitionFailed
}

func (s *AcquisitionService) resolve(ctx context.Context, op sourcing.AcquisitionOperation, publish, replayed bool) (sourcing.AcquisitionResult, error) {
	if op.State == sourcing.AcquisitionFailed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionFailed
	}
	if op.Command == nil || (op.State != sourcing.AcquisitionPublishing && op.State != sourcing.AcquisitionPublished) {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}
	if err := s.authorizeScope(ctx, op.Scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	var receipt sourcing.PublicationReceipt
	var err error
	if publish {
		receipt, err = s.publisher.Publish(ctx, *op.Command)
	} else {
		receipt, err = s.publisher.Verify(ctx, *op.Command)
	}
	if err != nil {
		if errors.Is(err, sourcing.ErrPublicationForbidden) {
			return sourcing.AcquisitionResult{}, err
		}
		if publish && (errors.Is(err, catalog.ErrStaleSnapshot) || errors.Is(err, sourcing.ErrSourcePublicationConflict) || errors.Is(err, sourcing.ErrInvalidSourcePublication)) {
			if finishErr := s.operations.Finish(ctx, op, sourcing.AcquisitionFailed, "PUBLICATION_CONFLICT"); finishErr != nil {
				return sourcing.AcquisitionResult{}, finishErr
			}
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionConflict
		}
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}
	persisted, err := s.exact(ctx, op)
	if err != nil {
		if errors.Is(err, sourcing.ErrPublicationForbidden) {
			return sourcing.AcquisitionResult{}, err
		}
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}
	if receipt.PublicationID != persisted.Receipt.PublicationID || receipt.InputHash != persisted.Receipt.InputHash || receipt.CatalogVersion != persisted.Receipt.CatalogVersion || receipt.OrganizationID != op.Scope.OrganizationID || receipt.ActorID != op.Scope.ActorID {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	if op.State != sourcing.AcquisitionPublished {
		if err := s.operations.Finish(ctx, op, sourcing.AcquisitionPublished, ""); err != nil {
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
		}
	}
	op.State = sourcing.AcquisitionPublished
	return sourcing.AcquisitionResult{Operation: op, Replayed: replayed, Publication: &persisted}, nil
}

func (s *AcquisitionService) exact(ctx context.Context, op sourcing.AcquisitionOperation) (sourcing.PersistedPublication, error) {
	persisted, err := s.publisher.Read(ctx, op.Command.PublicationID)
	if err != nil {
		return sourcing.PersistedPublication{}, err
	}
	r := persisted.Receipt
	if r.OrganizationID != op.Scope.OrganizationID || r.ActorID != op.Scope.ActorID || r.PublicationID != op.Command.PublicationID || r.CatalogPublicationID != op.Command.PublicationID || r.ProductKey != op.Command.ProductKey || r.CatalogVersion == 0 {
		return sourcing.PersistedPublication{}, sourcing.ErrAcquisitionUnavailable
	}
	return persisted, nil
}
