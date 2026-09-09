// Package sourcingpersistence atomically binds immutable source evidence to
// the Catalog-owned immutable snapshot version in one PostgreSQL transaction.
package sourcingpersistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/sourcing"
)

type CatalogBinding struct {
	Version       uint64
	PublicationID string
	SnapshotJSON  []byte
}

// CatalogBridge is implemented by the Catalog persistence owner at the app
// composition boundary. This repository never implements or copies Catalog's
// publication/version algorithm.
type CatalogBridge interface {
	Publish(context.Context, sourcing.AtomicPublication) (CatalogBinding, error)
	Read(context.Context, string, string, uint64) (CatalogBinding, bool, error)
	PublicationExists(context.Context, string, string, string) (bool, error)
}

type CatalogBridgeFactory func(*gorm.DB) (CatalogBridge, error)

type repository struct {
	db      *gorm.DB
	catalog CatalogBridgeFactory
	fault   func(string) error
	now     func() time.Time
}

func NewRepository(db *gorm.DB, catalog CatalogBridgeFactory) (sourcing.PublicationStore, error) {
	if db == nil || db.Dialector.Name() != "postgres" || catalog == nil {
		return nil, sourcing.ErrSourcePublicationUnavailable
	}
	return &repository{db: db, catalog: catalog, now: time.Now}, nil
}

// InstallSchema is an explicit empty-database initializer. Ordinary publish
// and verify requests never execute DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrSourcePublicationUnavailable
	}
	return db.AutoMigrate(&publicationRecord{}, &receiptRecord{})
}

func (r *repository) Publish(ctx context.Context, publication sourcing.AtomicPublication) (sourcing.PublicationReceipt, error) {
	if err := validateAtomic(publication); err != nil {
		return sourcing.PublicationReceipt{}, err
	}
	var receipt sourcing.PublicationReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := advisoryLock(tx, publication.OrganizationID, publication.PublicationID); err != nil {
			return err
		}
		existing, receiptFound, err := loadReceipt(tx, publication.OrganizationID, publication.PublicationID)
		if err != nil {
			return err
		}
		evidence, evidenceFound, err := loadEvidence(tx, publication.OrganizationID, publication.PublicationID)
		if err != nil {
			return err
		}
		if receiptFound || evidenceFound {
			if !receiptFound || !evidenceFound {
				return sourcing.ErrSourcePublicationStateInvalid
			}
			verified, err := r.verifyLoaded(tx, publication, existing, evidence)
			if err != nil {
				return err
			}
			receipt = verified
			return nil
		}
		catalogBridge, err := r.catalog(tx)
		if err != nil {
			return err
		}
		catalogExists, err := catalogBridge.PublicationExists(ctx, publication.OrganizationID, publication.ProductKey, publication.PublicationID)
		if err != nil {
			return err
		}
		if catalogExists {
			return sourcing.ErrSourcePublicationStateInvalid
		}
		if err := r.inject("before_catalog"); err != nil {
			return err
		}
		published, err := catalogBridge.Publish(ctx, publication)
		if err != nil {
			return catalogCallError{err}
		}
		if err := r.inject("after_catalog"); err != nil {
			return err
		}
		envelopeHash := digest(publication.EnvelopeJSON)
		snapshotHash := digest(publication.SnapshotJSON)
		// PostgreSQL timestamptz round-trips at microsecond precision. Normalize
		// before the first response so replay/restart returns the same receipt.
		publishedAt := r.now().UTC().Truncate(time.Microsecond)
		evidence = publicationRecord{
			OrganizationID: publication.OrganizationID, PublicationID: publication.PublicationID,
			InputHash: publication.InputHash, ProducerKind: publication.Producer.Kind, ProducerVersion: publication.Producer.Version,
			ProductKey: publication.ProductKey, ExpectedBaseVersion: cloneVersion(publication.ExpectedBaseVersion),
			EnvelopeHash: envelopeHash, SnapshotHash: snapshotHash,
			EnvelopeJSON: append([]byte(nil), publication.EnvelopeJSON...), SnapshotJSON: append([]byte(nil), publication.SnapshotJSON...),
			CatalogVersion: published.Version, CatalogPublicationID: published.PublicationID,
			ActorID: publication.ActorID, PublishedAt: publishedAt,
		}
		if err := tx.Create(&evidence).Error; err != nil {
			return err
		}
		if err := r.inject("after_evidence"); err != nil {
			return err
		}
		existing = receiptRecord{
			OrganizationID: publication.OrganizationID, PublicationID: publication.PublicationID,
			InputHash: publication.InputHash, ProducerKind: publication.Producer.Kind, ProducerVersion: publication.Producer.Version,
			ProductKey: publication.ProductKey, ExpectedBaseVersion: cloneVersion(publication.ExpectedBaseVersion),
			CatalogVersion: published.Version, CatalogPublicationID: published.PublicationID,
			EnvelopeHash: envelopeHash, SnapshotHash: snapshotHash,
			ActorID: publication.ActorID, PublishedAt: publishedAt,
		}
		if err := tx.Create(&existing).Error; err != nil {
			return err
		}
		if err := r.inject("before_commit"); err != nil {
			return err
		}
		receipt = receiptFromRecord(existing)
		return nil
	})
	if err != nil {
		return sourcing.PublicationReceipt{}, mapError(err)
	}
	if err := r.inject("after_commit"); err != nil {
		return sourcing.PublicationReceipt{}, fmt.Errorf("%w: %v", sourcing.ErrSourcePublicationOutcomeUnknown, err)
	}
	return receipt, nil
}

func (r *repository) Verify(ctx context.Context, publication sourcing.AtomicPublication) (sourcing.PublicationReceipt, error) {
	if err := validateAtomic(publication); err != nil {
		return sourcing.PublicationReceipt{}, err
	}
	var verified sourcing.PublicationReceipt
	err := r.readTransaction(ctx, func(tx *gorm.DB) error {
		receipt, receiptFound, err := loadReceipt(tx, publication.OrganizationID, publication.PublicationID)
		if err != nil {
			return err
		}
		evidence, evidenceFound, err := loadEvidence(tx, publication.OrganizationID, publication.PublicationID)
		if err != nil {
			return err
		}
		if !receiptFound && !evidenceFound {
			return sourcing.ErrSourcePublicationNotFound
		}
		if !receiptFound || !evidenceFound {
			return sourcing.ErrSourcePublicationStateInvalid
		}
		verified, err = r.verifyLoaded(tx, publication, receipt, evidence)
		return err
	})
	if err != nil {
		return sourcing.PublicationReceipt{}, mapError(err)
	}
	return verified, nil
}

func (r *repository) Read(ctx context.Context, organizationID, publicationID string) (sourcing.PersistedPublication, error) {
	if !authidentity.IsBoundedIdentifier(organizationID) || !authidentity.IsBoundedIdentifier(publicationID) {
		return sourcing.PersistedPublication{}, sourcing.ErrInvalidSourcePublication
	}
	var persisted sourcing.PersistedPublication
	err := r.readTransaction(ctx, func(tx *gorm.DB) error {
		receipt, receiptFound, err := loadReceipt(tx, organizationID, publicationID)
		if err != nil {
			return err
		}
		evidence, evidenceFound, err := loadEvidence(tx, organizationID, publicationID)
		if err != nil {
			return err
		}
		if !receiptFound && !evidenceFound {
			return sourcing.ErrSourcePublicationNotFound
		}
		if !receiptFound || !evidenceFound {
			return sourcing.ErrSourcePublicationStateInvalid
		}
		var envelope sourcing.SourceEnvelope
		if json.Unmarshal(evidence.EnvelopeJSON, &envelope) != nil || json.Unmarshal(evidence.SnapshotJSON, &persisted.Snapshot) != nil {
			return sourcing.ErrSourcePublicationStateInvalid
		}
		inputHash, err := sourcing.CanonicalPublicationInputHash(
			sourcing.ProducerDescriptor{Kind: evidence.ProducerKind, Version: evidence.ProducerVersion},
			envelope, evidence.EnvelopeJSON, evidence.ProductKey, evidence.ExpectedBaseVersion,
		)
		if err != nil || inputHash != evidence.InputHash || inputHash != receipt.InputHash {
			return sourcing.ErrSourcePublicationStateInvalid
		}
		publication := sourcing.AtomicPublication{
			OrganizationID: evidence.OrganizationID, ActorID: evidence.ActorID,
			PublicationID: evidence.PublicationID, InputHash: evidence.InputHash,
			Producer:   sourcing.ProducerDescriptor{Kind: evidence.ProducerKind, Version: evidence.ProducerVersion},
			ProductKey: evidence.ProductKey, ExpectedBaseVersion: cloneVersion(evidence.ExpectedBaseVersion),
			Envelope: envelope, EnvelopeJSON: evidence.EnvelopeJSON,
			Snapshot: persisted.Snapshot, SnapshotJSON: evidence.SnapshotJSON,
		}
		persisted.Receipt, err = r.verifyLoaded(tx, publication, receipt, evidence)
		persisted.Envelope = envelope
		return err
	})
	if err != nil {
		return sourcing.PersistedPublication{}, mapError(err)
	}
	return persisted, nil
}

func (r *repository) readTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

func (r *repository) verifyLoaded(db *gorm.DB, publication sourcing.AtomicPublication, receipt receiptRecord, evidence publicationRecord) (sourcing.PublicationReceipt, error) {
	if receipt.InputHash != evidence.InputHash {
		return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationStateInvalid
	}
	if receipt.InputHash != publication.InputHash {
		return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationConflict
	}
	envelopeHash, snapshotHash := digest(publication.EnvelopeJSON), digest(publication.SnapshotJSON)
	if receipt.OrganizationID != publication.OrganizationID || receipt.PublicationID != publication.PublicationID ||
		evidence.OrganizationID != publication.OrganizationID || evidence.PublicationID != publication.PublicationID ||
		receipt.ProductKey != publication.ProductKey || evidence.ProductKey != publication.ProductKey ||
		receipt.ProducerKind != publication.Producer.Kind || evidence.ProducerKind != publication.Producer.Kind ||
		receipt.ProducerVersion != publication.Producer.Version || evidence.ProducerVersion != publication.Producer.Version ||
		!sameVersion(receipt.ExpectedBaseVersion, publication.ExpectedBaseVersion) || !sameVersion(evidence.ExpectedBaseVersion, publication.ExpectedBaseVersion) ||
		receipt.CatalogVersion == 0 || receipt.CatalogVersion != evidence.CatalogVersion ||
		receipt.CatalogPublicationID != publication.PublicationID || receipt.CatalogPublicationID != evidence.CatalogPublicationID ||
		receipt.EnvelopeHash != envelopeHash || evidence.EnvelopeHash != envelopeHash ||
		receipt.SnapshotHash != snapshotHash || evidence.SnapshotHash != snapshotHash ||
		evidence.EnvelopeBytes > sourcing.MaxEncodedEnvelopeBytes || evidence.SnapshotBytes > sourcing.MaxEncodedSnapshotBytes ||
		len(evidence.EnvelopeJSON) == 0 || len(evidence.SnapshotJSON) == 0 ||
		digest(evidence.EnvelopeJSON) != evidence.EnvelopeHash || digest(evidence.SnapshotJSON) != evidence.SnapshotHash ||
		!receipt.PublishedAt.Equal(evidence.PublishedAt) {
		return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationStateInvalid
	}
	var storedEnvelope sourcing.SourceEnvelope
	var storedSnapshotJSON json.RawMessage
	if json.Unmarshal(evidence.EnvelopeJSON, &storedEnvelope) != nil || json.Unmarshal(evidence.SnapshotJSON, &storedSnapshotJSON) != nil {
		return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationStateInvalid
	}
	bridge, err := r.catalog(db)
	if err != nil {
		return sourcing.PublicationReceipt{}, err
	}
	published, found, err := bridge.Read(db.Statement.Context, publication.OrganizationID, publication.ProductKey, receipt.CatalogVersion)
	if err != nil {
		return sourcing.PublicationReceipt{}, catalogCallError{err}
	}
	if !found || published.PublicationID != publication.PublicationID || digest(published.SnapshotJSON) != snapshotHash {
		return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationStateInvalid
	}
	return receiptFromRecord(receipt), nil
}

func loadReceipt(db *gorm.DB, organizationID, publicationID string) (receiptRecord, bool, error) {
	var record receiptRecord
	err := db.Where("organization_id = ? AND publication_id = ?", organizationID, publicationID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return receiptRecord{}, false, nil
	}
	return record, err == nil, err
}

func loadEvidence(db *gorm.DB, organizationID, publicationID string) (publicationRecord, bool, error) {
	var record publicationRecord
	err := db.Select("organization_id,publication_id,input_hash,producer_kind,producer_version,product_key,expected_base_version,envelope_hash,snapshot_hash,catalog_version,catalog_publication_id,actor_id,published_at,octet_length(envelope_json) AS envelope_bytes,CASE WHEN octet_length(envelope_json) <= ? THEN envelope_json ELSE NULL END AS envelope_json,octet_length(snapshot_json) AS snapshot_bytes,CASE WHEN octet_length(snapshot_json) <= ? THEN snapshot_json ELSE NULL END AS snapshot_json", sourcing.MaxEncodedEnvelopeBytes, sourcing.MaxEncodedSnapshotBytes).
		Where("organization_id = ? AND publication_id = ?", organizationID, publicationID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return publicationRecord{}, false, nil
	}
	return record, err == nil, err
}

func advisoryLock(db *gorm.DB, organizationID, publicationID string) error {
	raw, _ := json.Marshal([]string{organizationID, publicationID})
	sum := sha256.Sum256(raw)
	return db.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(sum[:8]))).Error
}

func validateAtomic(publication sourcing.AtomicPublication) error {
	if !authidentity.IsBoundedIdentifier(publication.OrganizationID) || !authidentity.IsBoundedIdentifier(publication.ActorID) ||
		!authidentity.IsBoundedIdentifier(publication.PublicationID) || !authidentity.IsBoundedIdentifier(publication.ProductKey) ||
		!authidentity.IsBoundedIdentifier(publication.Producer.Kind) || !authidentity.IsBoundedIdentifier(publication.Producer.Version) ||
		publication.ExpectedBaseVersion != nil && *publication.ExpectedBaseVersion > math.MaxInt64 ||
		len(publication.InputHash) != sha256.Size*2 ||
		len(publication.EnvelopeJSON) == 0 || len(publication.EnvelopeJSON) > sourcing.MaxEncodedEnvelopeBytes ||
		len(publication.SnapshotJSON) == 0 || len(publication.SnapshotJSON) > sourcing.MaxEncodedSnapshotBytes {
		return sourcing.ErrInvalidSourcePublication
	}
	decoded, err := hex.DecodeString(publication.InputHash)
	if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != publication.InputHash {
		return sourcing.ErrInvalidSourcePublication
	}
	envelopeJSON, envelopeErr := json.Marshal(publication.Envelope)
	snapshotJSON, snapshotErr := json.Marshal(publication.Snapshot)
	if envelopeErr != nil || snapshotErr != nil || string(envelopeJSON) != string(publication.EnvelopeJSON) || string(snapshotJSON) != string(publication.SnapshotJSON) {
		return sourcing.ErrInvalidSourcePublication
	}
	normalized, err := sourcing.Normalize(publication.Envelope)
	if err != nil {
		return sourcing.ErrInvalidSourcePublication
	}
	normalizedJSON, err := json.Marshal(normalized)
	if err != nil || string(normalizedJSON) != string(publication.EnvelopeJSON) {
		return sourcing.ErrInvalidSourcePublication
	}
	derived, err := sourcing.ToSnapshot(normalized)
	if err != nil {
		return sourcing.ErrInvalidSourcePublication
	}
	derivedJSON, err := json.Marshal(derived)
	if err != nil || string(derivedJSON) != string(publication.SnapshotJSON) {
		return sourcing.ErrInvalidSourcePublication
	}
	inputHash, err := sourcing.CanonicalPublicationInputHash(publication.Producer, normalized, normalizedJSON, publication.ProductKey, publication.ExpectedBaseVersion)
	if err != nil || inputHash != publication.InputHash {
		return sourcing.ErrInvalidSourcePublication
	}
	return nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func receiptFromRecord(record receiptRecord) sourcing.PublicationReceipt {
	return sourcing.PublicationReceipt{
		OrganizationID: record.OrganizationID, PublicationID: record.PublicationID, InputHash: record.InputHash,
		Producer:   sourcing.ProducerDescriptor{Kind: record.ProducerKind, Version: record.ProducerVersion},
		ProductKey: record.ProductKey, ExpectedBaseVersion: cloneVersion(record.ExpectedBaseVersion),
		CatalogVersion: record.CatalogVersion, CatalogPublicationID: record.CatalogPublicationID,
		EnvelopeHash: record.EnvelopeHash, SnapshotHash: record.SnapshotHash,
		ActorID: record.ActorID, PublishedAt: record.PublishedAt.UTC(),
	}
}

func sameVersion(left, right *uint64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func cloneVersion(version *uint64) *uint64 {
	if version == nil {
		return nil
	}
	copy := *version
	return &copy
}

func (r *repository) inject(stage string) error {
	if r.fault == nil {
		return nil
	}
	return r.fault(stage)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var catalogErr catalogCallError
	if errors.As(err, &catalogErr) {
		return catalogErr.cause
	}
	for _, stable := range []error{context.Canceled, context.DeadlineExceeded, sourcing.ErrInvalidSourcePublication, sourcing.ErrPublicationForbidden, sourcing.ErrSourcePublicationConflict, sourcing.ErrSourcePublicationTooLarge, sourcing.ErrSourcePublicationNotFound, sourcing.ErrSourcePublicationStateInvalid, sourcing.ErrSourcePublicationUnavailable, sourcing.ErrSourcePublicationOutcomeUnknown} {
		if errors.Is(err, stable) {
			return err
		}
	}
	return fmt.Errorf("%w: %v", sourcing.ErrSourcePublicationUnavailable, err)
}

type catalogCallError struct{ cause error }

func (e catalogCallError) Error() string { return e.cause.Error() }
func (e catalogCallError) Unwrap() error { return e.cause }

var _ sourcing.PublicationStore = (*repository)(nil)
