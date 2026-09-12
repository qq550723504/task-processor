// Package acquisition persists bounded recovery commands, never Product facts.
package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/sourcing"
)

type Repository struct{ db *gorm.DB }

const table = "public.product_acquisition_operations"
const columns = `organization_id,actor_id,idempotency_key,operation_id,offer_id,source_url,fingerprint,state,fence,lease_until,
 CASE WHEN octet_length(command)<=2097152 THEN command END AS command,
 COALESCE(octet_length(command),0) AS command_bytes,command_hash,failure_code,capture_sha256`

const acquisitionSchemaShape = `WITH expected(position,name,kind,required) AS (VALUES
 (1,'organization_id','character varying(128)',true),(2,'actor_id','character varying(128)',true),
 (3,'idempotency_key','uuid',true),(4,'operation_id','uuid',true),(5,'offer_id','character varying(20)',true),
 (6,'source_url','character varying(128)',true),(7,'fingerprint','character varying(64)',true),(8,'state','character varying(16)',true),
 (9,'fence','bigint',true),(10,'lease_until','timestamp with time zone',true),(11,'command','bytea',false),
 (12,'command_hash','character varying(64)',true),(13,'failure_code','character varying(32)',true),
 (14,'capture_sha256','character varying(64)',true)),
 actual AS (SELECT attnum AS position,attname::text AS name,format_type(atttypid,atttypmod) AS kind,attnotnull AS required
 FROM pg_attribute WHERE attrelid='public.product_acquisition_operations'::regclass AND attnum>0 AND NOT attisdropped)
 SELECT NOT EXISTS(SELECT 1 FROM expected e FULL JOIN actual a USING(position) WHERE e.name IS DISTINCT FROM a.name OR e.kind IS DISTINCT FROM a.kind OR e.required IS DISTINCT FROM a.required)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.product_acquisition_operations'::regclass AND contype='p' AND conkey=ARRAY[1,2,3]::smallint[] AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.product_acquisition_operations'::regclass AND contype='u' AND conkey=ARRAY[1,2,4]::smallint[] AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_class WHERE oid='public.product_acquisition_operations'::regclass AND relkind='r' AND NOT relrowsecurity AND NOT relforcerowsecurity)
 AND EXISTS(SELECT 1 FROM pg_attrdef d JOIN pg_attribute a ON a.attrelid=d.adrelid AND a.attnum=d.adnum
 WHERE d.adrelid='public.product_acquisition_operations'::regclass AND a.attname='capture_sha256'
 AND pg_get_expr(d.adbin,d.adrelid)=$default$''::character varying$default$)`

type record struct {
	CaptureSHA256                                                                                string
	OrganizationID, ActorID, IdempotencyKey, OperationID, OfferID, SourceURL, Fingerprint, State string
	Fence                                                                                        int64
	LeaseUntil                                                                                   time.Time
	Command                                                                                      []byte
	CommandBytes                                                                                 int64
	CommandHash, FailureCode                                                                     string
}

// InstallSchema is explicit initialization for an empty dedicated Product DB.
// Neither repository construction nor a request invokes DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrAcquisitionUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS public.product_acquisition_operations (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,
 idempotency_key uuid NOT NULL,operation_id uuid NOT NULL,
 offer_id varchar(20) NOT NULL,source_url varchar(128) NOT NULL,
 fingerprint varchar(64) NOT NULL,state varchar(16) NOT NULL,
 fence bigint NOT NULL,lease_until timestamptz NOT NULL,
 command bytea,command_hash varchar(64) NOT NULL DEFAULT '',failure_code varchar(32) NOT NULL DEFAULT '',
 capture_sha256 varchar(64) NOT NULL DEFAULT '',
 PRIMARY KEY(organization_id,actor_id,idempotency_key),
 UNIQUE(organization_id,actor_id,operation_id),
 CONSTRAINT acq_fence_positive CHECK(fence>0),
 CONSTRAINT acq_command_bound CHECK(command IS NULL OR octet_length(command) BETWEEN 1 AND 2097152),
 CONSTRAINT acq_capture_digest CHECK(capture_sha256 = '' OR capture_sha256 ~ '^[0-9a-f]{64}$'),
 CONSTRAINT acq_state_command CHECK(
 (state='acquiring' AND command IS NULL AND command_hash='') OR
 (state IN ('prepared','publishing','published') AND command IS NOT NULL AND length(command_hash)=64) OR
 (state='failed' AND ((command IS NULL AND command_hash='') OR (command IS NOT NULL AND length(command_hash)=64)))))`).Error
}

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	var row record
	if err := db.WithContext(ctx).Raw("SELECT " + columns + " FROM " + table + " LIMIT 0").Scan(&row).Error; err != nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	var checks int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_constraint WHERE conrelid='public.product_acquisition_operations'::regclass AND convalidated AND conname IN ('acq_fence_positive','acq_command_bound','acq_state_command','acq_capture_digest')`).Scan(&checks).Error; err != nil || checks != 4 {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	var shape bool
	if err := db.WithContext(ctx).Raw(acquisitionSchemaShape).Scan(&shape).Error; err != nil || !shape {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	var definitions []struct{ Name, Definition string }
	if err := db.WithContext(ctx).Raw("SELECT conname AS name,pg_get_constraintdef(oid) AS definition FROM pg_constraint WHERE conrelid='public.product_acquisition_operations'::regclass AND conname IN ('acq_fence_positive','acq_command_bound','acq_state_command','acq_capture_digest')").Scan(&definitions).Error; err != nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	expectedDefinitions := map[string]string{
		"acq_capture_digest": `CHECK ((((capture_sha256)::text = ''::text) OR ((capture_sha256)::text ~ '^[0-9a-f]{64}$'::text)))`,
		"acq_fence_positive": "CHECK ((fence > 0))",
		"acq_command_bound":  "CHECK (((command IS NULL) OR ((octet_length(command) >= 1) AND (octet_length(command) <= 2097152))))",
		"acq_state_command":  `CHECK (((((state)::text = 'acquiring'::text) AND (command IS NULL) AND ((command_hash)::text = ''::text)) OR (((state)::text = ANY ((ARRAY['prepared'::character varying, 'publishing'::character varying, 'published'::character varying])::text[])) AND (command IS NOT NULL) AND (length((command_hash)::text) = 64)) OR (((state)::text = 'failed'::text) AND (((command IS NULL) AND ((command_hash)::text = ''::text)) OR ((command IS NOT NULL) AND (length((command_hash)::text) = 64))))))`,
	}
	for _, definition := range definitions {
		if expectedDefinitions[definition.Name] != definition.Definition {
			return nil, sourcing.ErrAcquisitionUnavailable
		}
	}
	return &Repository{db: db}, nil
}

// transaction deliberately distinguishes a confirmed claim from COMMIT loss.
// No caller receives a true claim on an uncertain transaction result.
func (r *Repository) transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return sourcing.ErrAcquisitionUnknown
	}
	return nil
}

func (r *Repository) Start(ctx context.Context, requested sourcing.AcquisitionOperation) (op sourcing.AcquisitionOperation, claim bool, err error) {
	if !validIdentity(requested) {
		return op, false, sourcing.ErrInvalidAcquisition
	}
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		// Scope-wide admission serializes all actors without a second capacity fact.
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", requested.Scope.OrganizationID).Error; e != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		var e error
		op, e = read(tx, requested.Scope, "idempotency_key", requested.Key, true)
		if e == nil {
			if op.Source != requested.Source || op.Fingerprint != requested.Fingerprint || op.CaptureSHA256 != requested.CaptureSHA256 {
				return sourcing.ErrAcquisitionConflict
			}
			if op.State != sourcing.AcquisitionAcquiring {
				return nil
			}
			result := tx.Exec("UPDATE "+table+" SET fence=fence+1,lease_until=clock_timestamp()+interval '30 seconds' WHERE organization_id=? AND actor_id=? AND idempotency_key=? AND state='acquiring' AND command IS NULL AND lease_until<clock_timestamp() AND fence=?", op.Scope.OrganizationID, op.Scope.ActorID, op.Key, op.Fence)
			if result.Error != nil {
				return sourcing.ErrAcquisitionUnavailable
			}
			claim = result.RowsAffected == 1
			if claim {
				op, e = read(tx, requested.Scope, "idempotency_key", requested.Key, false)
			}
			return e
		}
		if !errors.Is(e, sourcing.ErrAcquisitionNotFound) {
			return e
		}
		var count struct{ Total, Active int64 }
		if e := tx.Raw("SELECT count(*) AS total,count(*) FILTER(WHERE state IN ('acquiring','prepared','publishing')) AS active FROM "+table+" WHERE organization_id=?", requested.Scope.OrganizationID).Scan(&count).Error; e != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		if count.Total >= sourcing.MaxAcquisitionOperations || count.Active >= sourcing.MaxActiveAcquisitionOperations {
			return sourcing.ErrAcquisitionCapacity
		}
		if e := tx.Exec("INSERT INTO "+table+" (organization_id,actor_id,idempotency_key,operation_id,offer_id,source_url,fingerprint,capture_sha256,state,fence,lease_until) VALUES (?,?,?,?,?,?,?,?,'acquiring',1,clock_timestamp()+interval '30 seconds')", requested.Scope.OrganizationID, requested.Scope.ActorID, requested.Key, requested.ID, requested.Source.OfferID, requested.Source.URL, requested.Fingerprint, requested.CaptureSHA256).Error; e != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		op, e = read(tx, requested.Scope, "idempotency_key", requested.Key, false)
		claim = e == nil
		return e
	})
	if err != nil {
		claim = false
	}
	return
}

func (r *Repository) ByKey(ctx context.Context, scope sourcing.PublicationScope, key string) (sourcing.AcquisitionOperation, error) {
	return read(r.db.WithContext(ctx), scope, "idempotency_key", key, false)
}
func (r *Repository) ByID(ctx context.Context, scope sourcing.PublicationScope, id string) (sourcing.AcquisitionOperation, error) {
	return read(r.db.WithContext(ctx), scope, "operation_id", id, false)
}

func (r *Repository) Prepare(ctx context.Context, requested sourcing.AcquisitionOperation, cmd sourcing.PublicationCommand) (op sourcing.AcquisitionOperation, err error) {
	if !validCommand(requested, cmd) {
		return op, sourcing.ErrInvalidAcquisition
	}
	raw, e := json.Marshal(cmd)
	if e != nil || len(raw) > sourcing.MaxAcquisitionCommandBytes {
		return op, sourcing.ErrSourcePublicationTooLarge
	}
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		current, e := read(tx, requested.Scope, "idempotency_key", requested.Key, true)
		if e != nil {
			return e
		}
		if current.Fence != requested.Fence || current.State != sourcing.AcquisitionAcquiring || current.ID != requested.ID {
			return sourcing.ErrAcquisitionFence
		}
		if current.Source != requested.Source || current.Fingerprint != requested.Fingerprint || current.CaptureSHA256 != requested.CaptureSHA256 || !validCommand(current, cmd) {
			return sourcing.ErrAcquisitionConflict
		}
		result := tx.Exec("UPDATE "+table+" SET command=?,command_hash=?,state='prepared' WHERE organization_id=? AND actor_id=? AND idempotency_key=? AND state='acquiring' AND command IS NULL AND fence=? AND lease_until>clock_timestamp()", raw, digest(raw), current.Scope.OrganizationID, current.Scope.ActorID, current.Key, current.Fence)
		if result.Error != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		if result.RowsAffected != 1 {
			return sourcing.ErrAcquisitionFence
		}
		op, e = read(tx, requested.Scope, "idempotency_key", requested.Key, false)
		return e
	})
	return
}

func (r *Repository) Claim(ctx context.Context, requested sourcing.AcquisitionOperation) (op sourcing.AcquisitionOperation, claim bool, err error) {
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		var e error
		op, e = read(tx, requested.Scope, "idempotency_key", requested.Key, true)
		if e != nil {
			return e
		}
		if op.Fence != requested.Fence || op.ID != requested.ID {
			return sourcing.ErrAcquisitionFence
		}
		if op.State != sourcing.AcquisitionPrepared {
			return nil
		}
		result := tx.Exec("UPDATE "+table+" SET state='publishing' WHERE organization_id=? AND actor_id=? AND idempotency_key=? AND state='prepared' AND fence=?", op.Scope.OrganizationID, op.Scope.ActorID, op.Key, op.Fence)
		if result.Error != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		claim = result.RowsAffected == 1
		if claim {
			op.State = sourcing.AcquisitionPublishing
		}
		return nil
	})
	if err != nil {
		claim = false
	}
	return
}

func (r *Repository) Finish(ctx context.Context, requested sourcing.AcquisitionOperation, state, code string) error {
	if state != sourcing.AcquisitionPublished && state != sourcing.AcquisitionFailed {
		return sourcing.ErrInvalidAcquisition
	}
	if (state == sourcing.AcquisitionPublished && code != "") || (state == sourcing.AcquisitionFailed && !validFailure(code)) {
		return sourcing.ErrInvalidAcquisition
	}
	return r.transaction(ctx, func(tx *gorm.DB) error {
		op, e := read(tx, requested.Scope, "idempotency_key", requested.Key, true)
		if e != nil {
			return e
		}
		if op.Fence != requested.Fence || op.ID != requested.ID {
			return sourcing.ErrAcquisitionFence
		}
		if op.State == state && op.FailureCode == code {
			return nil
		}
		if op.State != sourcing.AcquisitionPublishing && !(op.State == sourcing.AcquisitionAcquiring && state == sourcing.AcquisitionFailed) {
			return sourcing.ErrAcquisitionFence
		}
		if e := tx.Exec("UPDATE "+table+" SET state=?,failure_code=? WHERE organization_id=? AND actor_id=? AND idempotency_key=? AND fence=?", state, code, op.Scope.OrganizationID, op.Scope.ActorID, op.Key, op.Fence).Error; e != nil {
			return sourcing.ErrAcquisitionUnavailable
		}
		return nil
	})
}

func read(db *gorm.DB, scope sourcing.PublicationScope, field, key string, lock bool) (sourcing.AcquisitionOperation, error) {
	var empty sourcing.AcquisitionOperation
	if !authidentity.IsBoundedIdentifier(scope.OrganizationID) || !authidentity.IsBoundedIdentifier(scope.ActorID) || !canonicalUUID(key) {
		return empty, sourcing.ErrInvalidAcquisition
	}
	query := "SELECT " + columns + " FROM " + table + " WHERE organization_id=? AND actor_id=? AND " + field + "=?"
	if lock {
		query += " FOR UPDATE"
	}
	var row record
	result := db.Raw(query, scope.OrganizationID, scope.ActorID, key).Scan(&row)
	if result.Error != nil {
		return empty, sourcing.ErrAcquisitionUnavailable
	}
	if result.RowsAffected != 1 {
		return empty, sourcing.ErrAcquisitionNotFound
	}
	op := sourcing.AcquisitionOperation{Scope: sourcing.PublicationScope{OrganizationID: row.OrganizationID, ActorID: row.ActorID}, Key: row.IdempotencyKey, ID: row.OperationID, Source: sourcing.AcquisitionSource{OfferID: row.OfferID, URL: row.SourceURL}, Fingerprint: row.Fingerprint, CaptureSHA256: row.CaptureSHA256, State: row.State, Fence: row.Fence, LeaseUntil: row.LeaseUntil, CommandHash: row.CommandHash, FailureCode: row.FailureCode}
	if !validIdentity(op) || op.Fence < 1 || row.CommandBytes > sourcing.MaxAcquisitionCommandBytes {
		return empty, sourcing.ErrAcquisitionUnavailable
	}
	if row.CommandBytes > 0 {
		if len(row.Command) == 0 || digest(row.Command) != row.CommandHash {
			return empty, sourcing.ErrAcquisitionUnavailable
		}
		var cmd sourcing.PublicationCommand
		decoder := json.NewDecoder(strings.NewReader(string(row.Command)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&cmd) != nil || !validCommand(op, cmd) {
			return empty, sourcing.ErrAcquisitionUnavailable
		}
		op.Command = &cmd
	}
	if (op.Command == nil) != (op.CommandHash == "") {
		return empty, sourcing.ErrAcquisitionUnavailable
	}
	switch op.State {
	case sourcing.AcquisitionAcquiring:
		if op.Command != nil || op.FailureCode != "" {
			return empty, sourcing.ErrAcquisitionUnavailable
		}
	case sourcing.AcquisitionPrepared, sourcing.AcquisitionPublishing, sourcing.AcquisitionPublished:
		if op.Command == nil || op.FailureCode != "" {
			return empty, sourcing.ErrAcquisitionUnavailable
		}
	case sourcing.AcquisitionFailed:
		if !validFailure(op.FailureCode) {
			return empty, sourcing.ErrAcquisitionUnavailable
		}
	default:
		return empty, sourcing.ErrAcquisitionUnavailable
	}
	return op, nil
}

func validIdentity(op sourcing.AcquisitionOperation) bool {
	if !authidentity.IsBoundedIdentifier(op.Scope.OrganizationID) || !authidentity.IsBoundedIdentifier(op.Scope.ActorID) || !canonicalUUID(op.Key) {
		return false
	}
	source, err := sourcing.Canonical1688Source(op.Source.URL)
	if err != nil || source != op.Source {
		return false
	}
	id, _ := json.Marshal([]string{op.Scope.OrganizationID, op.Scope.ActorID, op.Key})
	if op.CaptureSHA256 != "" {
		fingerprint, err := sourcing.BrowserAcquisitionFingerprint(source, op.CaptureSHA256)
		return err == nil && op.ID == uuid.NewSHA1(uuid.NameSpaceURL, id).String() && op.Fingerprint == fingerprint
	}
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", source.URL})
	return op.ID == uuid.NewSHA1(uuid.NameSpaceURL, id).String() && op.Fingerprint == digest(input)
}

func validCommand(op sourcing.AcquisitionOperation, cmd sourcing.PublicationCommand) bool {
	producer := sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"}
	metadata := cmd.Envelope.RawReference.Metadata
	if op.CaptureSHA256 != "" {
		producer.Kind = sourcing.BrowserAcquisitionProducerKind
		if metadata["capture_sha256"] != op.CaptureSHA256 || metadata["channel"] != "browser_capture" || metadata["parser_version"] != sourcing.BrowserCaptureParserVersion || metadata["contract_version"] != sourcing.AcquisitionContractVersion {
			return false
		}
	} else if metadata["channel"] != "public_http" || metadata["capture_sha256"] != "" {
		return false
	}
	if !validIdentity(op) || cmd.ExpectedBaseVersion == nil || *cmd.ExpectedBaseVersion > math.MaxInt64 || cmd.Producer != producer {
		return false
	}
	id := cmd.Envelope.Identity
	if id.SourceType != sourcing.SourceTypeCrawler || id.SourcePlatform != "1688" || id.SourceID != op.Source.OfferID || id.SourceURL != op.Source.URL || id.Platform != "" || id.Region != "" || id.ProductID != "" || id.StoreID != 0 || cmd.Envelope.Trace.SourceRunID != "acquisition:"+op.ID {
		return false
	}
	normalized, err := sourcing.NormalizePublicationEnvelope(cmd.Envelope)
	if err != nil {
		return false
	}
	a, err := json.Marshal(cmd.Envelope)
	if err != nil || len(a) > sourcing.MaxEncodedEnvelopeBytes {
		return false
	}
	b, err := json.Marshal(normalized)
	if err != nil || string(a) != string(b) {
		return false
	}
	productKey, publicationID, err := sourcing.PublicationIdentity(normalized)
	return err == nil && productKey == cmd.ProductKey && publicationID == cmd.PublicationID
}

func canonicalUUID(key string) bool {
	v, e := uuid.Parse(key)
	return e == nil && v != uuid.Nil && v.String() == key && v.Variant() == uuid.RFC4122
}
func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func validFailure(code string) bool {
	switch code {
	case "SOURCE_UNAVAILABLE", "INVALID_SOURCE", "SOURCE_TOO_LARGE", "PUBLICATION_CONFLICT":
		return true
	}
	return false
}

var _ sourcing.AcquisitionOperationStore = (*Repository)(nil)
