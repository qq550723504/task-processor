package acquisition

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/sourcing"
)

const chargeTable = "public.product_acquisition_charge_intents"
const chargeColumns = "organization_id,actor_id,operation_id,member_id,funding,reservation_id,terminal_kind,evidence_id,publication_id,input_hash,product_key,catalog_version,snapshot_hash,created_at"

type chargeRecord struct {
	OrganizationID, ActorID, OperationID, MemberID, Funding                      string
	ReservationID                                                                *string
	TerminalKind, EvidenceID, PublicationID, InputHash, ProductKey, SnapshotHash string
	CatalogVersion                                                               int64
	CreatedAt                                                                    time.Time
}

func installChargeSchema(db *gorm.DB) error {
	return db.Exec(`CREATE TABLE IF NOT EXISTS public.product_acquisition_charge_intents (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,operation_id uuid NOT NULL,
 member_id varchar(128) NOT NULL,funding varchar(64) NOT NULL,reservation_id uuid,
 terminal_kind varchar(16) NOT NULL DEFAULT '',evidence_id varchar(256) NOT NULL DEFAULT '',
 publication_id varchar(128) NOT NULL DEFAULT '',input_hash varchar(64) NOT NULL DEFAULT '',
 product_key varchar(128) NOT NULL DEFAULT '',catalog_version bigint NOT NULL DEFAULT 0,
 snapshot_hash varchar(64) NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (organization_id,actor_id,operation_id),UNIQUE(organization_id,operation_id),
 FOREIGN KEY (organization_id,actor_id,operation_id) REFERENCES public.product_acquisition_operations(organization_id,actor_id,operation_id) ON DELETE RESTRICT,
 CONSTRAINT acq_charge_member CHECK(member_id<>'' AND funding IN ('member_allocated','enterprise_unallocated')),
 CONSTRAINT acq_charge_terminal CHECK(
 (terminal_kind='' AND evidence_id='' AND publication_id='' AND input_hash='' AND product_key='' AND catalog_version=0 AND snapshot_hash='') OR
 (terminal_kind='failed_fenced' AND evidence_id<>'' AND publication_id='' AND input_hash='' AND product_key='' AND catalog_version=0 AND snapshot_hash='') OR
 (terminal_kind='succeeded' AND reservation_id IS NOT NULL AND evidence_id<>'' AND publication_id<>'' AND length(input_hash)=64 AND product_key<>'' AND catalog_version>0 AND length(snapshot_hash)=64)))`).Error
}
func verifyChargeSchema(ctx context.Context, db *gorm.DB) error {
	var row chargeRecord
	if err := db.WithContext(ctx).Raw("SELECT " + chargeColumns + " FROM " + chargeTable + " LIMIT 0").Scan(&row).Error; err != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`SELECT
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.product_acquisition_charge_intents'::regclass AND contype='p' AND conkey=ARRAY[1,2,3]::smallint[] AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.product_acquisition_charge_intents'::regclass AND contype='u' AND conkey=ARRAY[1,3]::smallint[] AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.product_acquisition_charge_intents'::regclass AND contype='f' AND confrelid='public.product_acquisition_operations'::regclass AND conkey=ARRAY[1,2,3]::smallint[] AND confkey=ARRAY[1,2,4]::smallint[] AND convalidated AND NOT condeferrable)
 AND (SELECT count(*)=2 FROM pg_constraint WHERE conrelid='public.product_acquisition_charge_intents'::regclass AND contype='c' AND convalidated AND conname IN ('acq_charge_member','acq_charge_terminal'))`).Scan(&ready).Error
	if err != nil || !ready {
		return sourcing.ErrAcquisitionUnavailable
	}
	return nil
}
func (r *Repository) RecordChargeIntent(ctx context.Context, op sourcing.AcquisitionOperation, intent sourcing.AcquisitionChargeIntent) error {
	if !matchesChargeIntent(op, intent) {
		return sourcing.ErrInvalidAcquisition
	}
	return r.transaction(ctx, func(tx *gorm.DB) error {
		current, err := read(tx, op.Scope, "operation_id", op.ID, true)
		if err != nil {
			return err
		}
		if current.Fence != op.Fence || current.State == sourcing.AcquisitionFailed {
			return sourcing.ErrAcquisitionFence
		}
		if !matchesChargeIntent(current, intent) {
			return sourcing.ErrAcquisitionConflict
		}
		if err := tx.Exec("INSERT INTO "+chargeTable+" (organization_id,actor_id,operation_id,member_id,funding) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING", op.Scope.OrganizationID, op.Scope.ActorID, op.ID, intent.MemberID, intent.Funding).Error; err != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		row, err := readCharge(tx, op.Scope.OrganizationID, op.ID, true)
		if err != nil {
			return err
		}
		if row.ActorID != intent.Scope.ActorID || row.MemberID != intent.MemberID || row.Funding != intent.Funding {
			return sourcing.ErrAcquisitionConflict
		}
		return nil
	})
}
func (r *Repository) BindChargeReservation(ctx context.Context, op sourcing.AcquisitionOperation, intent sourcing.AcquisitionChargeIntent, reservation string) error {
	if !matchesChargeIntent(op, intent) || !canonicalUUID(reservation) {
		return sourcing.ErrInvalidAcquisition
	}
	return r.transaction(ctx, func(tx *gorm.DB) error {
		current, err := read(tx, op.Scope, "operation_id", op.ID, true)
		if err != nil {
			return err
		}
		row, err := readCharge(tx, op.Scope.OrganizationID, op.ID, true)
		if err != nil {
			return err
		}
		if !matchesChargeIntent(current, intent) || row.ActorID != intent.Scope.ActorID || row.MemberID != intent.MemberID || row.Funding != intent.Funding {
			return sourcing.ErrAcquisitionConflict
		}
		if row.ReservationID != nil {
			if *row.ReservationID == reservation {
				return nil
			}
			return sourcing.ErrAcquisitionConflict
		}
		if current.Fence != op.Fence || (current.State != sourcing.AcquisitionAcquiring && current.State != sourcing.AcquisitionPrepared) || row.TerminalKind != "" {
			return sourcing.ErrAcquisitionFence
		}
		updated := tx.Exec("UPDATE "+chargeTable+" SET reservation_id=? WHERE organization_id=? AND actor_id=? AND operation_id=? AND reservation_id IS NULL AND terminal_kind=''", reservation, row.OrganizationID, row.ActorID, row.OperationID)
		if updated.Error != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		if updated.RowsAffected != 1 {
			return sourcing.ErrAcquisitionFence
		}
		return nil
	})
}
func (r *Repository) ReadChargeIntent(ctx context.Context, org, operation string) (sourcing.AcquisitionChargeIntent, error) {
	row, err := readCharge(r.db.WithContext(ctx), org, operation, false)
	if err != nil {
		return sourcing.AcquisitionChargeIntent{}, err
	}
	op, err := read(r.db.WithContext(ctx), sourcing.PublicationScope{OrganizationID: org, ActorID: row.ActorID}, "operation_id", operation, false)
	if err != nil {
		return sourcing.AcquisitionChargeIntent{}, err
	}
	if row.TerminalKind != "" && row.ReservationID == nil {
		return sourcing.AcquisitionChargeIntent{}, sourcing.ErrAcquisitionFailed
	}
	return chargeIntent(row, op), nil
}
func (r *Repository) ReadChargeProof(ctx context.Context, org, operation string) (sourcing.AcquisitionChargeProof, error) {
	row, err := readCharge(r.db.WithContext(ctx), org, operation, false)
	if err != nil {
		return sourcing.AcquisitionChargeProof{}, err
	}
	op, err := read(r.db.WithContext(ctx), sourcing.PublicationScope{OrganizationID: org, ActorID: row.ActorID}, "operation_id", operation, false)
	if err != nil {
		return sourcing.AcquisitionChargeProof{}, err
	}
	proof := sourcing.AcquisitionChargeProof{Intent: chargeIntent(row, op), State: "unknown"}
	if row.ReservationID == nil {
		return proof, nil
	}
	proof.ReservationID = *row.ReservationID
	if row.TerminalKind == "failed_fenced" && op.State == sourcing.AcquisitionFailed {
		proof.State, proof.EvidenceID = row.TerminalKind, row.EvidenceID
		return proof, nil
	}
	if row.TerminalKind == "succeeded" && op.State == sourcing.AcquisitionPublished && row.CatalogVersion > 0 {
		proof.State, proof.EvidenceID, proof.PublicationID, proof.InputHash, proof.ProductKey, proof.CatalogVersion, proof.SnapshotHash = row.TerminalKind, row.EvidenceID, row.PublicationID, row.InputHash, row.ProductKey, uint64(row.CatalogVersion), row.SnapshotHash
	}
	return proof, nil
}
func readCharge(db *gorm.DB, org, operation string, lock bool) (chargeRecord, error) {
	var row chargeRecord
	if !authidentity.IsBoundedIdentifier(org) || !canonicalUUID(operation) {
		return row, sourcing.ErrInvalidAcquisition
	}
	query := "SELECT " + chargeColumns + " FROM " + chargeTable + " WHERE organization_id=? AND operation_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	result := db.Raw(query, org, operation).Scan(&row)
	if result.Error != nil {
		return row, sourcing.ErrAcquisitionUnavailable
	}
	if result.RowsAffected != 1 {
		return row, sourcing.ErrAcquisitionUnknown
	}
	return row, nil
}
func chargeIntent(row chargeRecord, op sourcing.AcquisitionOperation) sourcing.AcquisitionChargeIntent {
	return sourcing.AcquisitionChargeIntent{Scope: op.Scope, OperationID: op.ID, MemberID: row.MemberID, Funding: row.Funding, Fingerprint: op.Fingerprint, Source: op.Source}
}
func matchesChargeIntent(op sourcing.AcquisitionOperation, intent sourcing.AcquisitionChargeIntent) bool {
	return validIdentity(op) && intent.Scope == op.Scope && intent.OperationID == op.ID && intent.Fingerprint == op.Fingerprint && intent.Source == op.Source && authidentity.IsBoundedIdentifier(intent.MemberID) && (intent.Funding == sourcing.AcquisitionFundingMember || intent.Funding == sourcing.AcquisitionFundingEnterprise)
}
func markChargeFailure(tx *gorm.DB, op sourcing.AcquisitionOperation, code string) error {
	return tx.Exec("UPDATE "+chargeTable+" SET terminal_kind='failed_fenced',evidence_id=? WHERE organization_id=? AND actor_id=? AND operation_id=? AND terminal_kind=''", "acquisition:"+op.ID+":failure:"+code, op.Scope.OrganizationID, op.Scope.ActorID, op.ID).Error
}

// The guard borrows SRC/Catalog's existing Product transaction. Its lock must
// precede SRC and Catalog locks; Complete writes proof in that same transaction.
type PublicationChargeGuard struct {
	tx          *gorm.DB
	op          sourcing.AcquisitionOperation
	charge      chargeRecord
	publication sourcing.AtomicPublication
}

func NewPublicationChargeGuard(tx *gorm.DB) *PublicationChargeGuard {
	return &PublicationChargeGuard{tx: tx}
}
func (g *PublicationChargeGuard) Lock(ctx context.Context, publication sourcing.AtomicPublication) error {
	if g.tx == nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	if _, ok := g.tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return sourcing.ErrAcquisitionUnavailable
	}
	operation := strings.TrimPrefix(publication.Envelope.Trace.SourceRunID, "acquisition:")
	if publication.Envelope.Trace.SourceRunID != "acquisition:"+operation || !canonicalUUID(operation) {
		return sourcing.ErrInvalidAcquisition
	}
	op, err := read(g.tx.WithContext(ctx), sourcing.PublicationScope{OrganizationID: publication.OrganizationID, ActorID: publication.ActorID}, "operation_id", operation, true)
	if err != nil {
		return err
	}
	if op.State != sourcing.AcquisitionPublishing && op.State != sourcing.AcquisitionPublished {
		return sourcing.ErrAcquisitionFence
	}
	row, err := readCharge(g.tx.WithContext(ctx), publication.OrganizationID, operation, true)
	if err != nil {
		return err
	}
	if row.ReservationID == nil || row.TerminalKind == "failed_fenced" || op.Command == nil {
		return sourcing.ErrAcquisitionFence
	}
	command := *op.Command
	envelopeJSON, err := json.Marshal(command.Envelope)
	if err != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	inputHash, err := sourcing.CanonicalPublicationInputHash(command.Producer, command.Envelope, envelopeJSON, command.ProductKey, command.ExpectedBaseVersion)
	if err != nil || inputHash != publication.InputHash || command.PublicationID != publication.PublicationID || command.ProductKey != publication.ProductKey || command.Producer != publication.Producer || string(envelopeJSON) != string(publication.EnvelopeJSON) {
		return sourcing.ErrAcquisitionConflict
	}
	g.op, g.charge, g.publication = op, row, publication
	return nil
}
func (g *PublicationChargeGuard) Complete(ctx context.Context, receipt sourcing.PublicationReceipt) error {
	if g.charge.ReservationID == nil || receipt.OrganizationID != g.op.Scope.OrganizationID || receipt.ActorID != g.op.Scope.ActorID || receipt.PublicationID != g.publication.PublicationID || receipt.InputHash != g.publication.InputHash || receipt.ProductKey != g.publication.ProductKey || receipt.CatalogVersion == 0 {
		return sourcing.ErrAcquisitionConflict
	}
	snapshotHash := digest(g.publication.SnapshotJSON)
	if g.charge.TerminalKind == "succeeded" {
		if g.charge.PublicationID != receipt.PublicationID || g.charge.InputHash != receipt.InputHash || g.charge.ProductKey != receipt.ProductKey || g.charge.CatalogVersion != int64(receipt.CatalogVersion) || g.charge.SnapshotHash != snapshotHash {
			return sourcing.ErrAcquisitionConflict
		}
		return nil
	}
	evidence := "acquisition:" + g.op.ID + ":catalog:" + strconv.FormatUint(receipt.CatalogVersion, 10) + ":" + snapshotHash
	updated := g.tx.WithContext(ctx).Exec("UPDATE "+chargeTable+" SET terminal_kind='succeeded',evidence_id=?,publication_id=?,input_hash=?,product_key=?,catalog_version=?,snapshot_hash=? WHERE organization_id=? AND actor_id=? AND operation_id=? AND reservation_id=? AND terminal_kind=''", evidence, receipt.PublicationID, receipt.InputHash, receipt.ProductKey, receipt.CatalogVersion, snapshotHash, g.op.Scope.OrganizationID, g.op.Scope.ActorID, g.op.ID, *g.charge.ReservationID)
	if updated.Error != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	if updated.RowsAffected != 1 {
		return sourcing.ErrAcquisitionFence
	}
	updated = g.tx.WithContext(ctx).Exec("UPDATE "+table+" SET state='published' WHERE organization_id=? AND actor_id=? AND operation_id=? AND state='publishing' AND fence=?", g.op.Scope.OrganizationID, g.op.Scope.ActorID, g.op.ID, g.op.Fence)
	if updated.Error != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	if updated.RowsAffected != 1 {
		return sourcing.ErrAcquisitionFence
	}
	return nil
}

var _ sourcing.AcquisitionChargeStore = (*Repository)(nil)
