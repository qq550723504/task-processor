package sourcing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
)

const (
	MaxEncodedEnvelopeBytes = 2 << 20
	MaxEncodedSnapshotBytes = 2 << 20
	PublicationTimeout      = 10 * time.Second

	ControlledSnapshotProducerKind    = "controlled_snapshot"
	ControlledSnapshotProducerVersion = "v1"
)

var (
	ErrInvalidSourcePublication        = errors.New("invalid source publication")
	ErrPublicationForbidden            = errors.New("source publication forbidden")
	ErrSourcePublicationTooLarge       = errors.New("source publication exceeds size limit")
	ErrSourcePublicationNotFound       = errors.New("source publication not found")
	ErrSourcePublicationStateInvalid   = errors.New("source publication state is invalid")
	ErrSourcePublicationUnavailable    = errors.New("source publication store is unavailable")
	ErrSourcePublicationOutcomeUnknown = errors.New("source publication commit outcome is unknown")
	ErrSourceProducerNotAdmitted       = errors.New("source producer is not admitted")
	ErrSourcePublicationConflict       = catalog.ErrPublicationConflict
)

type ProducerDescriptor struct {
	Kind    string
	Version string
}

// PublicationScope is returned only by a trusted, live authorization boundary.
// Organization identity is deliberately absent from PublicationCommand.
type PublicationScope struct {
	OrganizationID string
	ActorID        string
}

type PublicationCommand struct {
	PublicationID       string
	Producer            ProducerDescriptor
	ProductKey          string
	ExpectedBaseVersion *uint64
	Envelope            SourceEnvelope
}

// PublicationReceipt is the durable exact binding between source evidence and
// the Catalog-owned immutable version.
type PublicationReceipt struct {
	OrganizationID       string
	PublicationID        string
	InputHash            string
	Producer             ProducerDescriptor
	ProductKey           string
	ExpectedBaseVersion  *uint64
	CatalogVersion       uint64
	CatalogPublicationID string
	EnvelopeHash         string
	SnapshotHash         string
	ActorID              string
	PublishedAt          time.Time
}

// AtomicPublication carries already-normalized, bounded facts into the one-DB
// persistence adapter. Catalog publication remains delegated to Catalog's writer.
type AtomicPublication struct {
	OrganizationID      string
	ActorID             string
	PublicationID       string
	InputHash           string
	Producer            ProducerDescriptor
	ProductKey          string
	ExpectedBaseVersion *uint64
	Envelope            SourceEnvelope
	EnvelopeJSON        []byte
	Snapshot            catalog.ProductSnapshot
	SnapshotJSON        []byte
}

// PersistedPublication is the read-only evidence contract for exact downstream
// consumers such as REV-1. It is always bound to one verified Catalog version.
type PersistedPublication struct {
	Receipt  PublicationReceipt
	Envelope SourceEnvelope
	Snapshot catalog.ProductSnapshot
}

type PublicationAuthorizer interface {
	Authorize(context.Context) (PublicationScope, error)
}

type PublicationStore interface {
	Publish(context.Context, AtomicPublication) (PublicationReceipt, error)
	Verify(context.Context, AtomicPublication) (PublicationReceipt, error)
	Read(context.Context, string, string) (PersistedPublication, error)
}

// InternalProducer is the admitted in-process entry. It has no HTTP/provider
// trust boundary and cannot accept caller-supplied Organization identity.
type InternalProducer struct {
	authorizer PublicationAuthorizer
	store      PublicationStore
	admitted   map[ProducerDescriptor]struct{}
}

func NewInternalProducer(authorizer PublicationAuthorizer, store PublicationStore, admitted ...ProducerDescriptor) (*InternalProducer, error) {
	if authorizer == nil || store == nil || len(admitted) == 0 {
		return nil, ErrSourcePublicationUnavailable
	}
	allow := make(map[ProducerDescriptor]struct{}, len(admitted))
	for _, producer := range admitted {
		producer.Kind = strings.ToLower(strings.TrimSpace(producer.Kind))
		producer.Version = strings.TrimSpace(producer.Version)
		if !authidentity.IsBoundedIdentifier(producer.Kind) || !authidentity.IsBoundedIdentifier(producer.Version) {
			return nil, ErrInvalidSourcePublication
		}
		allow[producer] = struct{}{}
	}
	return &InternalProducer{authorizer: authorizer, store: store, admitted: allow}, nil
}

func (p *InternalProducer) Publish(ctx context.Context, command PublicationCommand) (PublicationReceipt, error) {
	ctx, cancel := publicationContext(ctx)
	defer cancel()
	publication, err := p.prepare(ctx, command)
	if err != nil {
		return PublicationReceipt{}, err
	}
	receipt, err := p.store.Publish(ctx, publication)
	if err != nil {
		return PublicationReceipt{}, err
	}
	if err := validatePublicationReceipt(publication, receipt); err != nil {
		return PublicationReceipt{}, err
	}
	return receipt, nil
}

// Verify performs fresh authorization and a read-only persistent-fact check.
// It is the recovery entry for response loss or an unknown COMMIT result.
func (p *InternalProducer) Verify(ctx context.Context, command PublicationCommand) (PublicationReceipt, error) {
	ctx, cancel := publicationContext(ctx)
	defer cancel()
	publication, err := p.prepare(ctx, command)
	if err != nil {
		return PublicationReceipt{}, err
	}
	receipt, err := p.store.Verify(ctx, publication)
	if err != nil {
		return PublicationReceipt{}, err
	}
	if err := validatePublicationReceipt(publication, receipt); err != nil {
		return PublicationReceipt{}, err
	}
	return receipt, nil
}

// Read returns one exact persisted publication after fresh authorization.
func (p *InternalProducer) Read(ctx context.Context, publicationID string) (PersistedPublication, error) {
	ctx, cancel := publicationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return PersistedPublication{}, err
	}
	if p == nil || p.authorizer == nil || p.store == nil {
		return PersistedPublication{}, ErrSourcePublicationUnavailable
	}
	scope, err := p.authorizer.Authorize(ctx)
	if err != nil {
		return PersistedPublication{}, err
	}
	publicationID = strings.TrimSpace(publicationID)
	if !authidentity.IsBoundedIdentifier(scope.OrganizationID) || !authidentity.IsBoundedIdentifier(scope.ActorID) || !authidentity.IsBoundedIdentifier(publicationID) {
		return PersistedPublication{}, ErrInvalidSourcePublication
	}
	persisted, err := p.store.Read(ctx, scope.OrganizationID, publicationID)
	if err != nil {
		return PersistedPublication{}, err
	}
	envelope, err := Normalize(persisted.Envelope)
	if err != nil {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	derived, err := ToSnapshot(envelope)
	if err != nil {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	derivedJSON, err := json.Marshal(derived)
	if err != nil {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	storedSnapshotJSON, err := json.Marshal(persisted.Snapshot)
	if err != nil || string(derivedJSON) != string(storedSnapshotJSON) {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	inputHash, err := CanonicalPublicationInputHash(persisted.Receipt.Producer, envelope, envelopeJSON, persisted.Receipt.ProductKey, persisted.Receipt.ExpectedBaseVersion)
	if err != nil || inputHash != persisted.Receipt.InputHash {
		return PersistedPublication{}, ErrSourcePublicationStateInvalid
	}
	publication := AtomicPublication{
		OrganizationID: scope.OrganizationID, ActorID: persisted.Receipt.ActorID,
		PublicationID: publicationID, InputHash: inputHash, Producer: persisted.Receipt.Producer,
		ProductKey: persisted.Receipt.ProductKey, ExpectedBaseVersion: cloneVersion(persisted.Receipt.ExpectedBaseVersion),
		Envelope: envelope, EnvelopeJSON: envelopeJSON, Snapshot: derived, SnapshotJSON: derivedJSON,
	}
	if err := validatePublicationReceipt(publication, persisted.Receipt); err != nil {
		return PersistedPublication{}, err
	}
	persisted.Envelope = envelope
	persisted.Snapshot = derived
	return persisted, nil
}

func publicationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, PublicationTimeout)
}

func (p *InternalProducer) prepare(ctx context.Context, command PublicationCommand) (AtomicPublication, error) {
	if err := ctx.Err(); err != nil {
		return AtomicPublication{}, err
	}
	if p == nil || p.authorizer == nil || p.store == nil {
		return AtomicPublication{}, ErrSourcePublicationUnavailable
	}
	scope, err := p.authorizer.Authorize(ctx)
	if err != nil {
		return AtomicPublication{}, err
	}
	if !authidentity.IsBoundedIdentifier(scope.OrganizationID) || !authidentity.IsBoundedIdentifier(scope.ActorID) {
		return AtomicPublication{}, ErrPublicationForbidden
	}

	command.PublicationID = strings.TrimSpace(command.PublicationID)
	command.ProductKey = strings.TrimSpace(command.ProductKey)
	command.Producer.Kind = strings.ToLower(strings.TrimSpace(command.Producer.Kind))
	command.Producer.Version = strings.TrimSpace(command.Producer.Version)
	if !authidentity.IsBoundedIdentifier(command.PublicationID) ||
		!authidentity.IsBoundedIdentifier(command.ProductKey) ||
		!authidentity.IsBoundedIdentifier(command.Producer.Kind) ||
		!authidentity.IsBoundedIdentifier(command.Producer.Version) ||
		command.ExpectedBaseVersion != nil && *command.ExpectedBaseVersion > math.MaxInt64 {
		return AtomicPublication{}, ErrInvalidSourcePublication
	}
	if _, admitted := p.admitted[command.Producer]; !admitted {
		return AtomicPublication{}, ErrSourceProducerNotAdmitted
	}

	envelope, err := Normalize(command.Envelope)
	if err != nil {
		return AtomicPublication{}, fmt.Errorf("%w: %v", ErrInvalidSourcePublication, err)
	}
	snapshot, err := ToSnapshot(envelope)
	if err != nil {
		return AtomicPublication{}, fmt.Errorf("%w: %v", ErrInvalidSourcePublication, err)
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return AtomicPublication{}, fmt.Errorf("%w: encode envelope: %v", ErrInvalidSourcePublication, err)
	}
	if len(envelopeJSON) == 0 || len(envelopeJSON) > MaxEncodedEnvelopeBytes {
		return AtomicPublication{}, ErrSourcePublicationTooLarge
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return AtomicPublication{}, fmt.Errorf("%w: encode snapshot: %v", ErrInvalidSourcePublication, err)
	}
	if len(snapshotJSON) == 0 || len(snapshotJSON) > MaxEncodedSnapshotBytes {
		return AtomicPublication{}, ErrSourcePublicationTooLarge
	}

	inputHash, err := CanonicalPublicationInputHash(command.Producer, envelope, envelopeJSON, command.ProductKey, command.ExpectedBaseVersion)
	if err != nil {
		return AtomicPublication{}, err
	}
	return AtomicPublication{
		OrganizationID: scope.OrganizationID, ActorID: scope.ActorID,
		PublicationID: command.PublicationID, InputHash: inputHash,
		Producer: command.Producer, ProductKey: command.ProductKey,
		ExpectedBaseVersion: cloneVersion(command.ExpectedBaseVersion), Envelope: envelope,
		EnvelopeJSON: envelopeJSON, Snapshot: snapshot, SnapshotJSON: snapshotJSON,
	}, nil
}

// CanonicalPublicationInputHash is shared with persistence corruption checks.
// The Organization/publication tuple is the key and is intentionally not part
// of the hash payload.
func CanonicalPublicationInputHash(producer ProducerDescriptor, envelope SourceEnvelope, envelopeJSON []byte, productKey string, expectedBaseVersion *uint64) (string, error) {
	canonical := struct {
		Producer            ProducerDescriptor
		Identity            SourceIdentity
		Envelope            json.RawMessage
		ProductKey          string
		ExpectedBaseVersion *uint64
	}{producer, envelope.Identity, envelopeJSON, productKey, expectedBaseVersion}
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: encode canonical input: %v", ErrInvalidSourcePublication, err)
	}
	inputSum := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(inputSum[:]), nil
}

func cloneVersion(version *uint64) *uint64 {
	if version == nil {
		return nil
	}
	copy := *version
	return &copy
}

func validatePublicationReceipt(publication AtomicPublication, receipt PublicationReceipt) error {
	if receipt.OrganizationID != publication.OrganizationID || receipt.PublicationID != publication.PublicationID ||
		receipt.InputHash != publication.InputHash || receipt.Producer != publication.Producer ||
		receipt.ProductKey != publication.ProductKey || !sameOptionalVersion(receipt.ExpectedBaseVersion, publication.ExpectedBaseVersion) ||
		receipt.CatalogVersion == 0 || receipt.CatalogPublicationID != publication.PublicationID ||
		receipt.EnvelopeHash != encodedDigest(publication.EnvelopeJSON) || receipt.SnapshotHash != encodedDigest(publication.SnapshotJSON) ||
		!authidentity.IsBoundedIdentifier(receipt.ActorID) || receipt.PublishedAt.IsZero() {
		return ErrSourcePublicationStateInvalid
	}
	return nil
}

func sameOptionalVersion(left, right *uint64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func encodedDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
