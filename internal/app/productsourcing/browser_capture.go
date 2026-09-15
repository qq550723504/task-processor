package productsourcing

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"task-processor/internal/authz"
	acquisitionpersistence "task-processor/internal/integration/persistence/product/acquisition"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	sourcingpersistence "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// BrowserCaptureService uses the same operation store and publication protocol.
// It has no provider: receiving or recovering a capture never fetches a page.
type BrowserCaptureService struct{ core *AcquisitionService }

func NewBrowserCaptureService(store sourcing.AcquisitionOperationStore, publisher sourcing.AcquisitionPublisher, reader catalog.CompleteSnapshotReader, authorizer sourcing.PublicationAuthorizer) (*BrowserCaptureService, error) {
	core, err := NewAcquisitionService(store, nil, publisher, reader, authorizer)
	if err != nil {
		return nil, err
	}
	return &BrowserCaptureService{core: core}, nil
}

// NewBrowserAcquisition admits the approved Browser descriptor through existing
// SRC-1/Catalog constructors. It performs no schema installation or provider IO.
func NewBrowserAcquisition(ctx context.Context, db *gorm.DB, live sourcing.LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*BrowserCaptureService, error) {
	if ctx == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	operations, err := acquisitionpersistence.NewRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	store, err := sourcingpersistence.NewRepository(db, newCatalogBridge)
	if err != nil {
		return nil, err
	}
	authorizer, err := sourcing.NewContextAuthorizer(live, permissions)
	if err != nil {
		return nil, err
	}
	producer, err := sourcing.NewInternalProducer(authorizer, store, sourcing.ProducerDescriptor{Kind: sourcing.BrowserAcquisitionProducerKind, Version: "v1"})
	if err != nil {
		return nil, err
	}
	reader, err := catalogpersistence.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return nil, err
	}
	return NewBrowserCaptureService(operations, producer, reader, authorizer)
}

func (s *BrowserCaptureService) request(ctx context.Context, key string, body []byte) (sourcing.AcquisitionOperation, sourcing.AcquisitionEvidence, error) {
	scope, err := s.core.authorizer.Authorize(ctx)
	if err != nil {
		return sourcing.AcquisitionOperation{}, sourcing.AcquisitionEvidence{}, err
	}
	if !canonicalAcquisitionKey(key) {
		return sourcing.AcquisitionOperation{}, sourcing.AcquisitionEvidence{}, sourcing.ErrInvalidAcquisition
	}
	capture, err := sourcing.ParseBrowserCapture(body)
	if err != nil {
		return sourcing.AcquisitionOperation{}, sourcing.AcquisitionEvidence{}, err
	}
	op := acquisitionOperation(scope, key, capture.Source)
	op.CaptureSHA256 = capture.PayloadSHA256
	op.Fingerprint, err = sourcing.BrowserAcquisitionFingerprint(op.Source, op.CaptureSHA256)
	return op, capture.Evidence, err
}

func (s *BrowserCaptureService) Capture(ctx context.Context, key string, body []byte) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	request, evidence, err := s.request(ctx, key, body)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	// Validate factual content before admitting an operation or consuming capacity.
	envelope, err := sourcing.MapAcquisitionEvidence(request.Source, evidence, "browser_capture", request.ID)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	envelope.RawReference.Metadata["capture_sha256"] = request.CaptureSHA256
	if s.core.reader == nil {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	op, claim, err := s.core.operations.Start(ctx, request)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if op.CaptureSHA256 != request.CaptureSHA256 {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionConflict
	}
	replayed := !claim
	if op.State == sourcing.AcquisitionAcquiring {
		if !claim {
			return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
		}
		if err := s.core.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		base := uint64(0)
		current, err := s.core.reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: op.Scope.OrganizationID, ProductKey: productKey})
		if err == nil {
			base = current.Version
		} else if !errors.Is(err, catalog.ErrSnapshotNotReady) {
			return sourcing.AcquisitionResult{}, err
		}
		if err := s.core.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		command := sourcing.PublicationCommand{PublicationID: publicationID, ProductKey: productKey, Producer: sourcing.ProducerDescriptor{Kind: sourcing.BrowserAcquisitionProducerKind, Version: "v1"}, ExpectedBaseVersion: &base, Envelope: envelope}
		op, err = s.core.operations.Prepare(ctx, op, command)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
	}
	if op.State == sourcing.AcquisitionFailed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionFailed
	}
	publishClaim := false
	if op.State == sourcing.AcquisitionPrepared {
		if err := s.core.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		op, publishClaim, err = s.core.operations.Claim(ctx, op)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
	}
	// POST alone may use the existing confirmed-claim/Finish orchestration.
	return s.core.resolve(ctx, op, publishClaim, replayed)
}

// Verify checks the original payload intent but never prepares, claims or writes.
// ByKey is the payload-free recovery path after a receiver reload.
func (s *BrowserCaptureService) Verify(ctx context.Context, key string, body []byte) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	request, _, err := s.request(ctx, key, body)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	op, err := s.core.operations.ByKey(ctx, request.Scope, key)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	return s.resolveReadOnly(ctx, request.Scope, op)
}

func (s *BrowserCaptureService) ByKey(ctx context.Context, key string) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	scope, err := s.core.authorizer.Authorize(ctx)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if !canonicalAcquisitionKey(key) {
		return sourcing.AcquisitionResult{}, sourcing.ErrInvalidAcquisition
	}
	op, err := s.core.operations.ByKey(ctx, scope, key)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if op.Key != key {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	return s.resolveReadOnly(ctx, scope, op)
}

func (s *BrowserCaptureService) Read(ctx context.Context, operationID string) (sourcing.AcquisitionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcing.AcquisitionTimeout)
	defer cancel()
	scope, err := s.core.authorizer.Authorize(ctx)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if !canonicalAcquisitionKey(operationID) {
		return sourcing.AcquisitionResult{}, sourcing.ErrInvalidAcquisition
	}
	op, err := s.core.operations.ByID(ctx, scope, operationID)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if op.ID != operationID {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	return s.resolveReadOnly(ctx, scope, op)
}

func (s *BrowserCaptureService) resolveReadOnly(ctx context.Context, scope sourcing.PublicationScope, op sourcing.AcquisitionOperation) (sourcing.AcquisitionResult, error) {
	if op.Scope != scope || !canonicalAcquisitionKey(op.Key) || op.ID != acquisitionOperation(scope, op.Key, op.Source).ID {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	fingerprint, err := sourcing.BrowserAcquisitionFingerprint(op.Source, op.CaptureSHA256)
	if err != nil || fingerprint != op.Fingerprint {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionConflict
	}
	if op.State == sourcing.AcquisitionFailed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionFailed
	}
	if op.Command == nil || op.State == sourcing.AcquisitionAcquiring {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}
	if op.State != sourcing.AcquisitionPrepared && op.State != sourcing.AcquisitionPublishing && op.State != sourcing.AcquisitionPublished {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	metadata := op.Command.Envelope.RawReference.Metadata
	if op.Command.Producer != (sourcing.ProducerDescriptor{Kind: sourcing.BrowserAcquisitionProducerKind, Version: "v1"}) || metadata["capture_sha256"] != op.CaptureSHA256 || metadata["channel"] != "browser_capture" || metadata["parser_version"] != sourcing.BrowserCaptureParserVersion || metadata["contract_version"] != sourcing.AcquisitionContractVersion {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	if err := s.core.authorizeScope(ctx, scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	receipt, err := s.core.publisher.Verify(ctx, *op.Command)
	if err != nil {
		if errors.Is(err, sourcing.ErrPublicationForbidden) {
			return sourcing.AcquisitionResult{}, err
		}
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}
	if err := s.core.authorizeScope(ctx, scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	persisted, err := s.core.exact(ctx, op)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if receipt.PublicationID != persisted.Receipt.PublicationID || receipt.InputHash != persisted.Receipt.InputHash || receipt.CatalogVersion != persisted.Receipt.CatalogVersion || receipt.OrganizationID != scope.OrganizationID || receipt.ActorID != scope.ActorID || receipt.Producer != op.Command.Producer {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	// This is a response projection of a verified receipt, not a staging update.
	op.State = sourcing.AcquisitionPublished
	return sourcing.AcquisitionResult{Operation: op, Replayed: true, Publication: &persisted}, nil
}
