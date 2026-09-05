package catalogpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	productcatalog "task-processor/internal/product/catalog"
)

type repository struct {
	db                      *gorm.DB
	maxEncodedSnapshotBytes int
	publishMu               sync.Mutex
}

type boundedSnapshotReader struct{ delegate *repository }

func NewRepository(db *gorm.DB) (productcatalog.Repository, error) {
	if db == nil {
		return nil, repositoryUnavailable("construct repository", errors.New("database is nil"))
	}
	return &repository{db: db}, nil
}

// NewBoundedSnapshotReader creates a read-only adapter whose raw persisted
// payload is bounded before hashing, unmarshalling, or defensive cloning. The
// shared Repository remains compatible with snapshots already accepted by the
// Catalog publication path.
func NewBoundedSnapshotReader(db *gorm.DB, maxEncodedSnapshotBytes int) (productcatalog.CompleteSnapshotReader, error) {
	if db == nil {
		return nil, repositoryUnavailable("construct bounded snapshot reader", errors.New("database is nil"))
	}
	if maxEncodedSnapshotBytes <= 0 {
		return nil, errors.New("max encoded snapshot bytes must be positive")
	}
	return &boundedSnapshotReader{delegate: &repository{db: db, maxEncodedSnapshotBytes: maxEncodedSnapshotBytes}}, nil
}

func (r *boundedSnapshotReader) GetCurrentSnapshot(ctx context.Context, identity productcatalog.SnapshotIdentity) (productcatalog.PublishedSnapshot, error) {
	return r.delegate.GetCurrentSnapshot(ctx, identity)
}

func (r *boundedSnapshotReader) GetSnapshot(ctx context.Context, identity productcatalog.SnapshotIdentity, version uint64) (productcatalog.PublishedSnapshot, error) {
	return r.delegate.GetSnapshot(ctx, identity, version)
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return repositoryUnavailable("migrate schema", errors.New("database is nil"))
	}
	return mapRepositoryError("migrate schema", db.AutoMigrate(&SnapshotVersionRecord{}, &SnapshotHeadRecord{}))
}

func (r *repository) PublishSnapshot(ctx context.Context, request productcatalog.PublishRequest) (productcatalog.PublishedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	if err := productcatalog.ValidatePublishRequest(request); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	payload, err := json.Marshal(request.Snapshot)
	if err != nil {
		return productcatalog.PublishedSnapshot{}, fmt.Errorf("%w: encode snapshot: %v", productcatalog.ErrInvalidSnapshot, err)
	}
	sum := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(sum[:])

	r.publishMu.Lock()
	defer r.publishMu.Unlock()

	var published productcatalog.PublishedSnapshot
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		initialHead := SnapshotHeadRecord{TenantID: request.Identity.TenantID, ProductKey: request.Identity.ProductKey}
		if createErr := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initialHead).Error; createErr != nil {
			return mapRepositoryError("ensure snapshot head", createErr)
		}

		var head SnapshotHeadRecord
		if headErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND product_key = ?", request.Identity.TenantID, request.Identity.ProductKey).
			Take(&head).Error; headErr != nil {
			return mapRepositoryError("load snapshot head", headErr)
		}

		existing, found, loadErr := loadPublication(tx, request.Identity, request.PublicationID)
		if loadErr != nil {
			return loadErr
		}
		if found {
			if existing.PayloadHash != payloadHash {
				return productcatalog.ErrPublicationConflict
			}
			decoded, decodeErr := publishedFromRecord(existing, 0)
			if decodeErr != nil {
				return decodeErr
			}
			published = decoded
			return nil
		}

		nextVersion := head.CurrentVersion + 1

		record := SnapshotVersionRecord{
			TenantID: request.Identity.TenantID, ProductKey: request.Identity.ProductKey,
			Version: nextVersion, PublicationID: request.PublicationID,
			PayloadHash: payloadHash, SnapshotJSON: payload,
		}
		if createErr := tx.Create(&record).Error; createErr != nil {
			return mapRepositoryError("insert snapshot version", createErr)
		}
		if updateErr := tx.Model(&SnapshotHeadRecord{}).
			Where("tenant_id = ? AND product_key = ?", request.Identity.TenantID, request.Identity.ProductKey).
			Updates(map[string]any{"current_version": nextVersion, "publication_id": request.PublicationID}).Error; updateErr != nil {
			return mapRepositoryError("advance snapshot head", updateErr)
		}
		published = productcatalog.PublishedSnapshot{
			Identity: request.Identity, Version: nextVersion,
			PublicationID: request.PublicationID, Snapshot: request.Snapshot,
		}
		return nil
	})
	if err != nil {
		return productcatalog.PublishedSnapshot{}, mapRepositoryError("publish snapshot transaction", err)
	}
	return clonePublishedSnapshot(published)
}

func (r *repository) GetCurrentSnapshot(ctx context.Context, identity productcatalog.SnapshotIdentity) (productcatalog.PublishedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	if err := productcatalog.ValidateSnapshotIdentity(identity); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	var head SnapshotHeadRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND product_key = ?", identity.TenantID, identity.ProductKey).
		Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return productcatalog.PublishedSnapshot{}, productcatalog.ErrSnapshotNotReady
	}
	if err != nil {
		return productcatalog.PublishedSnapshot{}, mapRepositoryError("load current snapshot head", err)
	}
	published, err := r.GetSnapshot(ctx, identity, head.CurrentVersion)
	if err != nil {
		if errors.Is(err, productcatalog.ErrSnapshotNotReady) {
			return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("load current snapshot", errors.New("head version is missing"))
		}
		return productcatalog.PublishedSnapshot{}, err
	}
	if published.PublicationID != head.PublicationID {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("load current snapshot", errors.New("head publication does not match version"))
	}
	return published, nil
}

func (r *repository) GetSnapshot(ctx context.Context, identity productcatalog.SnapshotIdentity, version uint64) (productcatalog.PublishedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	if err := productcatalog.ValidateSnapshotIdentity(identity); err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	if version == 0 {
		return productcatalog.PublishedSnapshot{}, productcatalog.ErrSnapshotNotReady
	}
	var record SnapshotVersionRecord
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND product_key = ? AND version = ?", identity.TenantID, identity.ProductKey, version)
	if r.maxEncodedSnapshotBytes > 0 {
		sizeExpression := boundedSnapshotSizeExpression(query.Dialector.Name())
		projection := fmt.Sprintf(
			"tenant_id, product_key, version, publication_id, payload_hash, CASE WHEN %s <= ? THEN snapshot_json ELSE NULL END AS snapshot_json",
			sizeExpression,
		)
		query = query.Select(projection, r.maxEncodedSnapshotBytes)
	}
	err := query.Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return productcatalog.PublishedSnapshot{}, productcatalog.ErrSnapshotNotReady
	}
	if err != nil {
		return productcatalog.PublishedSnapshot{}, mapRepositoryError("load snapshot version", err)
	}
	return publishedFromRecord(record, r.maxEncodedSnapshotBytes)
}

func boundedSnapshotSizeExpression(dialect string) string {
	if dialect == "postgres" {
		return "OCTET_LENGTH(snapshot_json::text)"
	}
	return "LENGTH(snapshot_json)"
}

func loadPublication(tx *gorm.DB, identity productcatalog.SnapshotIdentity, publicationID string) (SnapshotVersionRecord, bool, error) {
	var record SnapshotVersionRecord
	err := tx.Where("tenant_id = ? AND product_key = ? AND publication_id = ?", identity.TenantID, identity.ProductKey, publicationID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SnapshotVersionRecord{}, false, nil
	}
	if err != nil {
		return SnapshotVersionRecord{}, false, mapRepositoryError("load snapshot publication", err)
	}
	return record, true, nil
}

func publishedFromRecord(record SnapshotVersionRecord, maxEncodedSnapshotBytes int) (productcatalog.PublishedSnapshot, error) {
	if len(record.SnapshotJSON) == 0 || (maxEncodedSnapshotBytes > 0 && len(record.SnapshotJSON) > maxEncodedSnapshotBytes) {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("decode snapshot publication", errors.New("encoded snapshot exceeds reader size limit"))
	}
	decodedHash, err := hex.DecodeString(record.PayloadHash)
	if err != nil || len(decodedHash) != sha256.Size || hex.EncodeToString(decodedHash) != record.PayloadHash {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("decode snapshot publication", errors.New("payload hash is not canonical SHA-256 hex"))
	}
	sum := sha256.Sum256(record.SnapshotJSON)
	if hex.EncodeToString(sum[:]) != record.PayloadHash {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("decode snapshot publication", errors.New("snapshot payload hash mismatch"))
	}
	var snapshot productcatalog.ProductSnapshot
	if err := json.Unmarshal(record.SnapshotJSON, &snapshot); err != nil {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("decode snapshot publication", err)
	}
	if _, err := productcatalog.CloneProductSnapshot(snapshot); err != nil {
		return productcatalog.PublishedSnapshot{}, repositoryStateInvalid("validate snapshot publication", err)
	}
	return productcatalog.PublishedSnapshot{
		Identity: productcatalog.SnapshotIdentity{TenantID: record.TenantID, ProductKey: record.ProductKey},
		Version:  record.Version, PublicationID: record.PublicationID, Snapshot: snapshot,
	}, nil
}

func clonePublishedSnapshot(published productcatalog.PublishedSnapshot) (productcatalog.PublishedSnapshot, error) {
	cloned, err := productcatalog.CloneProductSnapshot(published.Snapshot)
	if err != nil {
		return productcatalog.PublishedSnapshot{}, err
	}
	published.Snapshot = cloned
	return published, nil
}

func mapRepositoryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	for _, stable := range []error{
		productcatalog.ErrInvalidSnapshot,
		productcatalog.ErrInvalidPublication,
		productcatalog.ErrPublicationConflict,
		productcatalog.ErrSnapshotNotReady,
		productcatalog.ErrRepositoryUnavailable,
		productcatalog.ErrRepositoryStateInvalid,
	} {
		if errors.Is(err, stable) {
			return err
		}
	}
	return repositoryUnavailable(operation, err)
}

func repositoryUnavailable(operation string, cause error) error {
	return fmt.Errorf("%w: %s: %v", productcatalog.ErrRepositoryUnavailable, operation, cause)
}

func repositoryStateInvalid(operation string, cause error) error {
	return fmt.Errorf("%w: %s: %v", productcatalog.ErrRepositoryStateInvalid, operation, cause)
}

var _ productcatalog.Repository = (*repository)(nil)
var _ productcatalog.CompleteSnapshotReader = (*boundedSnapshotReader)(nil)
