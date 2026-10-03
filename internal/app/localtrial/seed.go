// Package localtrial prepares one deterministic synthetic sample through the
// current Product, Review, Asset, Store and Listing owners. It is used only by
// the one-shot #36 loopback trial installer, never by request-time serving.
package localtrial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	recordstore "task-processor/internal/app/listingrecordstore"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/draft"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
)

const (
	sampleProductKey     = "issue36-local-product"
	samplePublicationID  = "issue36-local-publication-v1"
	sampleEvidenceID     = "issue36-local-evidence-v1"
	sampleProposalKey    = "issue36-local-proposal-v1"
	sampleListingKey     = "issue36-local-listing-v1"
	sampleStoreID        = "81111111-1111-4111-8111-111111111111"
	sampleStoreCreateKey = "82222222-2222-4222-8222-222222222222"
	sampleAssetActionID  = "83333333-3333-4333-8333-333333333333"
	sampleAssetID        = "84444444-4444-4444-8444-444444444444"
	sampleAssetRunID     = "85555555-5555-4555-8555-555555555555"
)

type Sample struct {
	ProductKey string
	StoreID    string
	ProposalID string
	RecordID   string
}

type setupAccess struct{ organizationID, actorID string }

func (access setupAccess) ResolveLiveRoles(_ context.Context, organizationID, actorID string) ([]string, error) {
	if organizationID != access.organizationID || actorID != access.actorID {
		return nil, sourcing.ErrPublicationForbidden
	}
	return []string{"listingkit_admin"}, nil
}

type storeReference struct{ repository storecenter.Repository }

func (r storeReference) GetStoreReference(ctx context.Context, organizationID, storeID string) (record.StoreReference, error) {
	stored, err := r.repository.Get(ctx, organizationID, storeID)
	if errors.Is(err, storecenter.ErrNotFound) {
		return record.StoreReference{}, record.ErrNotFound
	}
	if err != nil {
		return record.StoreReference{}, record.ErrUnavailable
	}
	if stored == nil || stored.OrganizationID() != organizationID || stored.ID() != storeID {
		return record.StoreReference{}, record.ErrNotFound
	}
	return record.StoreReference{OrganizationID: organizationID, StoreID: storeID, Platform: string(stored.Platform())}, nil
}

func PrepareSample(ctx context.Context, db *gorm.DB, organizationID, actorID string) (Sample, error) {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || !authidentity.IsBoundedIdentifier(organizationID) || !authidentity.IsBoundedIdentifier(actorID) {
		return Sample{}, errors.New("local trial sample dependencies unavailable")
	}
	if err := ctx.Err(); err != nil {
		return Sample{}, err
	}
	identity := authidentity.AuthenticatedIdentity{
		TenantID: organizationID, EffectiveOrganizationID: organizationID, HomeOrganizationID: organizationID,
		UserID: actorID, Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(5 * time.Minute),
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, identity)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		return Sample{}, err
	}
	source, err := productsourcing.NewInternalProducer(db, setupAccess{organizationID: organizationID, actorID: actorID}, authorizer)
	if err != nil {
		return Sample{}, err
	}
	envelope := sourcing.SourceEnvelope{
		Identity:         sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "local-synthetic", SourceID: sampleProductKey, SourceVersion: "v1"},
		RawReference:     sourcing.RawSourceReference{ReferenceType: "captured", ReferenceID: sampleEvidenceID, SnapshotID: "issue36-local-snapshot-v1", Checksum: sourcing.RawSnapshotChecksum("issue36-local-synthetic-source-v1"), CapturedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)},
		ProductCandidate: sourcing.ProductCandidate{Title: "Synthetic trial bottle", Description: "Synthetic local sample for title review and listing preparation", Brand: "Local Trial", CategoryPath: []string{"Home", "Drinkware"}, Variants: []sourcing.ProductVariantCandidate{{SourceID: "variant-1", SKU: "TRIAL-BOTTLE", Price: 12, Currency: "USD", Stock: 3}}},
		AssetCandidates:  []sourcing.AssetCandidate{{SourceID: "source-image-1", URL: "https://fixtures.invalid/issue36/source.png", MediaType: "image", Role: "main", Width: 1, Height: 1}},
	}
	publication, err := source.Publish(ctx, sourcing.PublicationCommand{
		PublicationID: samplePublicationID,
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    sampleProductKey, Envelope: envelope,
	})
	if err != nil {
		return Sample{}, err
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	if err != nil {
		return Sample{}, err
	}
	reviewRepository, err := reviewstore.NewRepository(db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	if err != nil {
		return Sample{}, err
	}
	reviews, err := review.NewCandidateService(reader, source, reviewRepository, authorizer)
	if err != nil {
		return Sample{}, err
	}
	proposal, err := reviews.CreateFromCandidate(ctx, sampleProposalKey, review.CandidateInput{
		Base:          review.CreateInput{ProductKey: sampleProductKey, BaseVersion: publication.CatalogVersion},
		PublicationID: publication.CatalogPublicationID, PolicyVersion: "title-review-v1",
		Candidate: enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Synthetic improved trial bottle", EvidenceIDs: []string{sampleEvidenceID}}}},
	})
	if err != nil {
		return Sample{}, err
	}
	storeRepository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		return Sample{}, err
	}
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{
		ID: sampleStoreID, OrganizationID: organizationID, ActorSubject: actorID, Name: "Synthetic SHEIN trial store", Platform: "shein", Region: "US",
		ExternalStoreID: "issue36-synthetic", CreateIdempotencyKey: sampleStoreCreateKey, OccurredAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return Sample{}, err
	}
	if _, _, err = storeRepository.CreateOrReplay(ctx, organizationID, store); err != nil {
		return Sample{}, err
	}
	assets, err := assetstore.NewRepository(db)
	if err != nil {
		return Sample{}, err
	}
	_, err = assets.CommitApproval(ctx, productasset.ApprovalCommit{
		TenantID: organizationID, ProductKey: sampleProductKey, TargetPlatform: "shein", ActionID: sampleAssetActionID, SourceSnapshotVersion: publication.CatalogVersion,
		Assets: []productasset.ApprovedAsset{{ID: sampleAssetID, RunID: sampleAssetRunID, PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://fixtures.invalid/issue36/approved-main.jpg"}},
	})
	if err != nil {
		return Sample{}, err
	}
	approved, err := assetstore.NewBoundedApprovedInventoryReader(db, record.MaxPayloadBytes)
	if err != nil {
		return Sample{}, err
	}
	records, err := recordstore.NewRepository(db, authorizer)
	if err != nil {
		return Sample{}, err
	}
	listing, err := record.NewService(record.ServiceDependencies{
		Products: reader, Assets: approved, Stores: storeReference{repository: storeRepository}, Records: records,
		Builder: draft.Builder{}, Evaluator: sheinvalidator.ExactApprovedAssetValidator{}, Authorizer: authorizer,
		Now: time.Now, RuleRevision: sheinvalidator.DiagnosticRuleVersion, PolicyRevision: sheinvalidator.BindingVersion,
	})
	if err != nil {
		return Sample{}, err
	}
	receipt, err := listing.Create(ctx, sampleListingKey, record.Input{
		ProductKey: sampleProductKey, SnapshotVersion: publication.CatalogVersion, StoreID: sampleStoreID,
		Country: "US", Language: "en", Action: contract.SaveDraft,
	})
	if err != nil {
		return Sample{}, err
	}
	return Sample{ProductKey: sampleProductKey, StoreID: sampleStoreID, ProposalID: proposal.ID, RecordID: receipt.RecordID}, nil
}
