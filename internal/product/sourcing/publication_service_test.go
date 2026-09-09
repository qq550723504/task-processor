package sourcing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type admissionFunc func(context.Context) (PublicationScope, error)

func (f admissionFunc) Authorize(ctx context.Context) (PublicationScope, error) { return f(ctx) }

type publicationStoreStub struct {
	publishCalls int
	verifyCalls  int
	last         AtomicPublication
	receipt      PublicationReceipt
	err          error
	deadline     time.Time
}

func (s *publicationStoreStub) Publish(ctx context.Context, publication AtomicPublication) (PublicationReceipt, error) {
	s.publishCalls++
	s.last = publication
	s.deadline, _ = ctx.Deadline()
	return s.result(publication), s.err
}

func (s *publicationStoreStub) Verify(ctx context.Context, publication AtomicPublication) (PublicationReceipt, error) {
	s.verifyCalls++
	s.last = publication
	s.deadline, _ = ctx.Deadline()
	return s.result(publication), s.err
}

func (s *publicationStoreStub) result(publication AtomicPublication) PublicationReceipt {
	result := s.receipt
	if result.OrganizationID == "" {
		result.OrganizationID = publication.OrganizationID
	}
	if result.PublicationID == "" {
		result.PublicationID = publication.PublicationID
	}
	result.InputHash = publication.InputHash
	result.Producer = publication.Producer
	if result.ProductKey == "" {
		result.ProductKey = publication.ProductKey
	}
	result.ExpectedBaseVersion = cloneVersion(publication.ExpectedBaseVersion)
	if result.CatalogVersion == 0 {
		result.CatalogVersion = 1
	}
	result.CatalogPublicationID = publication.PublicationID
	result.EnvelopeHash = encodedDigest(publication.EnvelopeJSON)
	result.SnapshotHash = encodedDigest(publication.SnapshotJSON)
	result.ActorID = publication.ActorID
	result.PublishedAt = time.Now().UTC()
	return result
}

func (s *publicationStoreStub) Read(context.Context, string, string) (PersistedPublication, error) {
	return PersistedPublication{}, s.err
}

func validPublicationCommand() PublicationCommand {
	base := uint64(0)
	return PublicationCommand{
		PublicationID:       "pub-1",
		Producer:            ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"},
		ProductKey:          "product-1",
		ExpectedBaseVersion: &base,
		Envelope: SourceEnvelope{
			Identity:         SourceIdentity{SourceType: SourceTypeManualImport, SourcePlatform: "controlled", SourceID: "source-1", SourceVersion: "v1"},
			RawReference:     RawSourceReference{ReferenceType: "snapshot", ReferenceID: "raw-1", Checksum: "sha256:abc"},
			ProductCandidate: ProductCandidate{Title: "Bottle", Attributes: map[string]string{"color": "blue"}},
			Warnings:         []SourceWarning{{Code: " MISSING_WEIGHT ", Field: " weight ", Message: " unavailable "}},
			MissingFacts:     []MissingFact{{Field: " weight ", Reason: " source omitted "}},
			Trace:            SourceTrace{SourceRunID: "run-1", RequestID: "request-1"},
		},
	}
}

func TestInternalProducerPublishesCanonicalEvidenceWithTenSecondDeadline(t *testing.T) {
	store := &publicationStoreStub{receipt: PublicationReceipt{OrganizationID: "org-a", PublicationID: "pub-1", ProductKey: "product-1", CatalogVersion: 1}}
	producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
		return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)

	before := time.Now()
	receipt, err := producer.Publish(context.Background(), validPublicationCommand())
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.CatalogVersion)
	require.Equal(t, 1, store.publishCalls)
	require.Equal(t, "org-a", store.last.OrganizationID)
	require.Equal(t, "actor-a", store.last.ActorID)
	require.Equal(t, "missing_weight", store.last.Envelope.Warnings[0].Code)
	require.Equal(t, "weight", store.last.Envelope.MissingFacts[0].Field)
	require.NotEmpty(t, store.last.InputHash)
	require.NotEmpty(t, store.last.EnvelopeJSON)
	require.NotEmpty(t, store.last.SnapshotJSON)
	require.LessOrEqual(t, store.deadline.Sub(before), PublicationTimeout+time.Second)
	require.Greater(t, store.deadline.Sub(before), PublicationTimeout-time.Second)
}

func TestInternalProducerCanonicalHashCoversProducerIdentityEnvelopeTargetAndBase(t *testing.T) {
	store := &publicationStoreStub{}
	producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
		return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)

	base := validPublicationCommand()
	_, err = producer.Publish(context.Background(), base)
	require.NoError(t, err)
	baseline := store.last.InputHash
	require.Len(t, baseline, 64)

	changedProducerHash, err := CanonicalPublicationInputHash(ProducerDescriptor{Kind: "other", Version: "v2"}, store.last.Envelope, store.last.EnvelopeJSON, store.last.ProductKey, store.last.ExpectedBaseVersion)
	require.NoError(t, err)
	require.NotEqual(t, baseline, changedProducerHash)

	mutations := []func(*PublicationCommand){
		func(v *PublicationCommand) { v.Envelope.Identity.SourceID = "source-2" },
		func(v *PublicationCommand) { v.Envelope.ProductCandidate.Title = "Cup" },
		func(v *PublicationCommand) { v.ProductKey = "product-2" },
		func(v *PublicationCommand) { n := uint64(1); v.ExpectedBaseVersion = &n },
	}
	for _, mutate := range mutations {
		candidate := validPublicationCommand()
		mutate(&candidate)
		_, err = producer.Publish(context.Background(), candidate)
		require.NoError(t, err)
		require.NotEqual(t, baseline, store.last.InputHash)
	}

	// Go JSON canonicalizes map keys; caller map insertion order is not identity.
	equivalent := validPublicationCommand()
	equivalent.Envelope.ProductCandidate.Attributes = map[string]string{"size": "m", "color": "blue"}
	_, err = producer.Publish(context.Background(), equivalent)
	require.NoError(t, err)
	first := store.last.InputHash
	equivalent.Envelope.ProductCandidate.Attributes = map[string]string{"color": "blue", "size": "m"}
	_, err = producer.Publish(context.Background(), equivalent)
	require.NoError(t, err)
	require.Equal(t, first, store.last.InputHash)
}

func TestInternalProducerReauthorizesPublishAndReadOnlyVerify(t *testing.T) {
	authorized := true
	authCalls := 0
	store := &publicationStoreStub{}
	producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
		authCalls++
		if !authorized {
			return PublicationScope{}, ErrPublicationForbidden
		}
		return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)

	_, err = producer.Publish(context.Background(), validPublicationCommand())
	require.NoError(t, err)
	authorized = false
	_, err = producer.Publish(context.Background(), validPublicationCommand())
	require.ErrorIs(t, err, ErrPublicationForbidden)
	_, err = producer.Verify(context.Background(), validPublicationCommand())
	require.ErrorIs(t, err, ErrPublicationForbidden)
	require.Equal(t, 3, authCalls)
	require.Equal(t, 1, store.publishCalls)
	require.Equal(t, 0, store.verifyCalls)
}

func TestInternalProducerRejectsInvalidScopeAndOversizedFactsBeforeStore(t *testing.T) {
	for _, scope := range []PublicationScope{{}, {OrganizationID: "org-a"}, {ActorID: "actor-a"}, {OrganizationID: "bad org", ActorID: "actor-a"}} {
		store := &publicationStoreStub{}
		producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) { return scope, nil }), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
		require.NoError(t, err)
		_, err = producer.Publish(context.Background(), validPublicationCommand())
		require.ErrorIs(t, err, ErrPublicationForbidden)
		require.Zero(t, store.publishCalls)
	}
	store := &publicationStoreStub{}
	producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
		return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)
	unadmitted := validPublicationCommand()
	unadmitted.Producer.Version = "v2"
	_, err = producer.Publish(context.Background(), unadmitted)
	require.ErrorIs(t, err, ErrSourceProducerNotAdmitted)
	require.Zero(t, store.publishCalls)

	for _, mutate := range []func(*PublicationCommand){
		func(v *PublicationCommand) { v.PublicationID = "" },
		func(v *PublicationCommand) { v.ProductKey = strings.Repeat("p", 129) },
		func(v *PublicationCommand) { v.Producer.Kind = "" },
		func(v *PublicationCommand) {
			v.Envelope.RawReference.Metadata = map[string]string{"raw": strings.Repeat("x", MaxEncodedEnvelopeBytes)}
		},
		func(v *PublicationCommand) {
			v.Envelope.ProductCandidate.Description = strings.Repeat("x", MaxEncodedSnapshotBytes)
		},
	} {
		store := &publicationStoreStub{}
		producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
			return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
		}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
		require.NoError(t, err)
		command := validPublicationCommand()
		mutate(&command)
		_, err = producer.Publish(context.Background(), command)
		require.Error(t, err)
		require.Zero(t, store.publishCalls)
	}
}

func TestInternalProducerAcceptsExactEnvelopeAndSnapshotLimits(t *testing.T) {
	newProducer := func(store *publicationStoreStub) *InternalProducer {
		producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
			return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
		}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
		require.NoError(t, err)
		return producer
	}

	t.Run("envelope", func(t *testing.T) {
		store := &publicationStoreStub{}
		command := validPublicationCommand()
		command.Envelope.SupplierOrCostFacts.Facts = map[string]string{"padding": ""}
		normalized, err := Normalize(command.Envelope)
		require.NoError(t, err)
		encoded, err := json.Marshal(normalized)
		require.NoError(t, err)
		command.Envelope.SupplierOrCostFacts.Facts["padding"] = strings.Repeat("x", MaxEncodedEnvelopeBytes-len(encoded))
		_, err = newProducer(store).Publish(context.Background(), command)
		require.NoError(t, err)
		require.Len(t, store.last.EnvelopeJSON, MaxEncodedEnvelopeBytes)
		require.Less(t, len(store.last.SnapshotJSON), MaxEncodedSnapshotBytes)
	})

	t.Run("snapshot", func(t *testing.T) {
		store := &publicationStoreStub{}
		command := validPublicationCommand()
		command.Envelope.Warnings = []SourceWarning{{Code: "source_warning", Field: "field", Message: "x"}}
		snapshot, err := ToSnapshot(command.Envelope)
		require.NoError(t, err)
		encoded, err := json.Marshal(snapshot)
		require.NoError(t, err)
		delta := MaxEncodedSnapshotBytes - len(encoded)
		require.Positive(t, delta)
		if delta%2 != 0 {
			command.Envelope.Warnings[0].Field += "x"
			delta--
		}
		command.Envelope.Warnings[0].Message += strings.Repeat("x", delta/2)
		_, err = newProducer(store).Publish(context.Background(), command)
		require.NoError(t, err)
		require.Len(t, store.last.SnapshotJSON, MaxEncodedSnapshotBytes)
		require.Less(t, len(store.last.EnvelopeJSON), MaxEncodedEnvelopeBytes)
	})
}

func TestInternalProducerPreservesEarlierCancellation(t *testing.T) {
	store := &publicationStoreStub{}
	producer, err := NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) {
		return PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = producer.Publish(ctx, validPublicationCommand())
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, store.publishCalls)

	dependencyErr := errors.New("live authorization unavailable")
	producer, err = NewInternalProducer(admissionFunc(func(context.Context) (PublicationScope, error) { return PublicationScope{}, dependencyErr }), store, ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"})
	require.NoError(t, err)
	_, err = producer.Publish(context.Background(), validPublicationCommand())
	require.ErrorIs(t, err, dependencyErr)
}
