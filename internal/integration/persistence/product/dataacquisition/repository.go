package dataacquisitionpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/dataservice"
	keystore "task-processor/internal/integration/persistence/dataservice"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
)

// Publisher participates in the caller's Product transaction. Its implementation
// composes the current SRC/Catalog writers; it must never commit independently.
type Publisher func(context.Context, *gorm.DB, dataacquisition.Job, dataacquisition.Item) (collection.Source, error)
type ResultReaderFactory func(*gorm.DB) (dataacquisition.CapturedResultReader, error)
type Repository struct {
	db           *gorm.DB
	live         dataacquisition.LiveAccess
	publish      Publisher
	resultReader ResultReaderFactory
}

func NewRepository(ctx context.Context, db *gorm.DB, live dataacquisition.LiveAccess, publish Publisher, resultReader ResultReaderFactory) (*Repository, error) {
	if live == nil || publish == nil || resultReader == nil {
		return nil, dataacquisition.ErrUnavailable
	}
	if err := VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	if err := keystore.VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	if err := collectionstore.VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	return &Repository{db: db, live: live, publish: publish, resultReader: resultReader}, nil
}

// WithResultRead serializes complete API result materialization with credential
// changes. All supplied readers reuse this transaction's connection, including
// installations with a one-connection Product pool.
func (r *Repository) WithResultRead(ctx context.Context, p dataacquisition.Principal, read func(context.Context, dataacquisition.ResultReadRepository, dataacquisition.CapturedResultReader) error) error {
	if p.Scope.Validate() != nil || !collection.ValidID(p.CredentialID) || p.CredentialRevision < 1 || read == nil {
		return dataacquisition.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		key, err := readKeyForLock(tx, p.Scope, p.CredentialID, "FOR SHARE")
		if err != nil {
			return err
		}
		allowed := false
		for _, permission := range key.Input.Permissions {
			allowed = allowed || permission == dataservice.PermissionResult
		}
		check := func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			var current time.Time
			if err := tx.Raw("SELECT clock_timestamp()").Scan(&current).Error; err != nil {
				return err
			}
			if key.ID != p.CredentialID || key.Scope != p.Scope || key.Revision != p.CredentialRevision || key.State != "ACTIVE" || !current.Before(key.Input.ExpiresAt) || !allowed {
				return dataacquisition.ErrForbidden
			}
			return nil
		}
		if err = check(); err != nil {
			return err
		}
		reader, err := r.resultReader(tx)
		if err != nil || reader == nil {
			return dataacquisition.ErrUnavailable
		}
		bound := &Repository{db: tx, live: r.live, publish: r.publish, resultReader: r.resultReader}
		if err = read(ctx, bound, reader); err != nil {
			return err
		}
		return check()
	})
}

type jobRow struct {
	OrganizationID, ActorID, MemberID, ID, CommandKey, InputHash, CredentialID, Funding, State, Reason string
	CredentialRevision                                                                                 int64
	QueryJSON                                                                                          []byte
	Discovered, Canceled                                                                               bool
	QuotaReserved                                                                                      int
	DayWindow, MonthWindow, CreatedAt, Deadline                                                        time.Time
}

const jobColumns = "organization_id,actor_id,member_id,id,command_key,input_hash,COALESCE(credential_id::text,'') AS credential_id,COALESCE(credential_revision,0) AS credential_revision,query_json,funding,state,reason,discovered,canceled,quota_reserved,day_window,month_window,created_at,deadline"

func (row jobRow) job() (dataacquisition.Job, error) {
	var q dataacquisition.Query
	if json.Unmarshal(row.QueryJSON, &q) != nil {
		return dataacquisition.Job{}, dataacquisition.ErrUnavailable
	}
	normalized, err := dataacquisition.NormalizeQuery(q)
	if err != nil || collection.Digest(normalized) != collection.Digest(q) {
		return dataacquisition.Job{}, dataacquisition.ErrUnavailable
	}
	return dataacquisition.Job{ID: row.ID, Scope: collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}, CredentialID: row.CredentialID, CommandKey: row.CommandKey, InputHash: row.InputHash, Query: q, Funding: orgresource.ResourceFunding(row.Funding), State: row.State, Reason: row.Reason, Discovered: row.Discovered, Canceled: row.Canceled, CreatedAt: row.CreatedAt, Deadline: row.Deadline}, nil
}
func readJob(db *gorm.DB, s collection.Scope, id string, lock bool) (jobRow, error) {
	query := "SELECT " + jobColumns + " FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? AND id=?"
	if lock {
		query += " FOR UPDATE"
	}
	var row jobRow
	result := db.Raw(query, s.OrganizationID, s.ActorID, id).Scan(&row)
	if result.Error != nil {
		return row, result.Error
	}
	if result.RowsAffected != 1 {
		return row, dataacquisition.ErrNotFound
	}
	return row, nil
}

type itemRow struct {
	OrganizationID, ActorID, JobID, ID, ASIN, State, ClaimToken, ReservationID, ChargeState, Reason, TerminalEvidence string
	IntentJSON, EvidenceJSON, SourceJSON                                                                              []byte
	LeaseUntil                                                                                                        *time.Time
}

const itemColumns = "organization_id,actor_id,job_id,id,asin,state,COALESCE(claim_token::text,'') AS claim_token,lease_until,intent_json,evidence_json,source_json,COALESCE(terminal_evidence::text,'') AS terminal_evidence,COALESCE(reservation_id::text,'') AS reservation_id,charge_state,reason"

func (row itemRow) item() (dataacquisition.Item, error) {
	item := dataacquisition.Item{ID: row.ID, ASIN: row.ASIN, State: row.State, ClaimToken: row.ClaimToken, ReservationID: row.ReservationID, ChargeState: orgresource.ReservationState(row.ChargeState), Reason: row.Reason}
	if len(row.EvidenceJSON) > 0 {
		var e dataacquisition.Evidence
		if json.Unmarshal(row.EvidenceJSON, &e) != nil || e.Validate() != nil {
			return item, dataacquisition.ErrUnavailable
		}
		item.Evidence = &e
	}
	if len(row.SourceJSON) > 0 {
		var s collection.Source
		if json.Unmarshal(row.SourceJSON, &s) != nil || s.Kind != "amazon_data" || s.OperationID != item.ID || s.Version == 0 {
			return item, dataacquisition.ErrUnavailable
		}
		item.Source = &s
	}
	return item, nil
}
func (row itemRow) intent() (orgresource.ConsumerChargeIntent, error) {
	var intent orgresource.ConsumerChargeIntent
	if json.Unmarshal(row.IntentJSON, &intent) != nil || !orgresource.ValidConsumerChargeIntent(intent) || intent.Identity.OrganizationID != row.OrganizationID || intent.Identity.OperationID != row.ID || intent.ActorID != row.ActorID || intent.Identity.Consumer != orgresource.ConsumerAmazonData {
		return intent, dataacquisition.ErrUnavailable
	}
	return intent, nil
}
func readItem(db *gorm.DB, job dataacquisition.Job, id string, lock bool) (itemRow, error) {
	query := "SELECT " + itemColumns + " FROM data_acquisition_items WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?"
	if lock {
		query += " FOR UPDATE"
	}
	var row itemRow
	result := db.Raw(query, job.Scope.OrganizationID, job.Scope.ActorID, job.ID, id).Scan(&row)
	if result.Error != nil {
		return row, result.Error
	}
	if result.RowsAffected != 1 {
		return row, dataacquisition.ErrNotFound
	}
	return row, nil
}
func itemRows(db *gorm.DB, job dataacquisition.Job) ([]itemRow, error) {
	var rows []itemRow
	err := db.Raw("SELECT "+itemColumns+" FROM data_acquisition_items WHERE organization_id=? AND actor_id=? AND job_id=? ORDER BY id", job.Scope.OrganizationID, job.Scope.ActorID, job.ID).Scan(&rows).Error
	return rows, err
}
func summarize(db *gorm.DB, row jobRow) (dataacquisition.Job, error) {
	job, err := row.job()
	if err != nil {
		return job, err
	}
	items, err := itemRows(db, job)
	if err != nil {
		return job, err
	}
	for _, i := range items {
		switch i.State {
		case "SAVED":
			job.Saved++
			if i.ChargeState == string(orgresource.ReservationCommitted) {
				job.ConfirmedFen += dataacquisition.PriceFen
			} else {
				job.PendingFen += dataacquisition.PriceFen
			}
		case "FAILED":
			job.Failed++
		default:
			job.Pending++
		}
	}
	if job.Saved > 0 {
		batch, err := collection.NewPublicationBatch(job.Scope, job.ID, "amazon_data", "Amazon · "+job.Query.Site)
		if err != nil {
			return job, err
		}
		job.BatchID = batch.ID
	}
	return job, nil
}
func (r *Repository) Read(ctx context.Context, s collection.Scope, id string) (dataacquisition.Job, error) {
	if s.Validate() != nil || !collection.ValidID(id) {
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	row, err := readJob(r.db.WithContext(ctx), s, id, false)
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return summarize(r.db.WithContext(ctx), row)
}
func (r *Repository) List(ctx context.Context, s collection.Scope, limit int) ([]dataacquisition.Job, error) {
	if s.Validate() != nil || limit < 1 || limit > 100 {
		return nil, dataacquisition.ErrInvalid
	}
	var rows []jobRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+jobColumns+" FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? ORDER BY created_at DESC,id LIMIT ?", s.OrganizationID, s.ActorID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	jobs := []dataacquisition.Job{}
	for _, row := range rows {
		job, err := summarize(r.db.WithContext(ctx), row)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func lockKey(tx *gorm.DB, s collection.Scope, id string) (dataservice.Credential, error) {
	return readKeyForLock(tx, s, id, "FOR UPDATE")
}
func readKeyForLock(tx *gorm.DB, s collection.Scope, id, lock string) (dataservice.Credential, error) {
	if id == "" {
		return dataservice.Credential{}, nil
	}
	var raw struct {
		MemberID, State string
		Revision        int64
		ConfigJSON      []byte
		ExpiresAt       time.Time
	}
	result := tx.Raw("SELECT member_id,state,revision,config_json,expires_at FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND id=? "+lock, s.OrganizationID, s.ActorID, id).Scan(&raw)
	if result.Error != nil {
		return dataservice.Credential{}, result.Error
	}
	if result.RowsAffected != 1 {
		return dataservice.Credential{}, dataacquisition.ErrForbidden
	}
	var input dataservice.KeyInput
	if json.Unmarshal(raw.ConfigJSON, &input) != nil || !input.ExpiresAt.Truncate(time.Microsecond).Equal(raw.ExpiresAt) {
		return dataservice.Credential{}, dataacquisition.ErrUnavailable
	}
	return dataservice.Credential{ID: id, Scope: collection.Scope{OrganizationID: s.OrganizationID, ActorID: s.ActorID, MemberID: raw.MemberID}, State: raw.State, Revision: raw.Revision, Input: input}, nil
}
func activeKey(tx *gorm.DB, key dataservice.Credential, s collection.Scope) error {
	if key.ID == "" {
		return nil
	}
	var current time.Time
	if err := tx.Raw("SELECT clock_timestamp()").Scan(&current).Error; err != nil {
		return err
	}
	allowed := false
	for _, p := range key.Input.Permissions {
		allowed = allowed || p == dataservice.PermissionAcquire
	}
	if key.Scope != s || key.State != "ACTIVE" || !current.Before(key.Input.ExpiresAt) || !allowed {
		return dataacquisition.ErrForbidden
	}
	return nil
}
func lockQuota(tx *gorm.DB, row jobRow, create bool) error {
	if row.CredentialID == "" {
		return nil
	}
	for _, window := range []struct {
		kind  string
		start time.Time
	}{{"day", row.DayWindow}, {"month", row.MonthWindow}} {
		if create {
			if err := tx.Exec("INSERT INTO data_service_quota(organization_id,actor_id,key_id,window_kind,window_start) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING", row.OrganizationID, row.ActorID, row.CredentialID, window.kind, window.start).Error; err != nil {
				return err
			}
		}
		var found int64
		result := tx.Raw("SELECT consumed_rows FROM data_service_quota WHERE organization_id=? AND actor_id=? AND key_id=? AND window_kind=? AND window_start=? FOR UPDATE", row.OrganizationID, row.ActorID, row.CredentialID, window.kind, window.start).Scan(&found)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return dataacquisition.ErrUnavailable
		}
	}
	return nil
}
func quotaMove(tx *gorm.DB, row *jobRow, release, consume int) error {
	if release < 0 || consume < 0 || consume > release || release > row.QuotaReserved {
		return dataacquisition.ErrConflict
	}
	if row.CredentialID != "" {
		for _, window := range []struct {
			kind  string
			start time.Time
		}{{"day", row.DayWindow}, {"month", row.MonthWindow}} {
			result := tx.Exec("UPDATE data_service_quota SET reserved_rows=reserved_rows-?,reserved_fen=reserved_fen-?,consumed_rows=consumed_rows+?,consumed_fen=consumed_fen+? WHERE organization_id=? AND actor_id=? AND key_id=? AND window_kind=? AND window_start=? AND reserved_rows>=? AND reserved_fen>=?", release, int64(release)*dataacquisition.PriceFen, consume, int64(consume)*dataacquisition.PriceFen, row.OrganizationID, row.ActorID, row.CredentialID, window.kind, window.start, release, int64(release)*dataacquisition.PriceFen)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return dataacquisition.ErrConflict
			}
		}
	}
	row.QuotaReserved -= release
	return tx.Exec("UPDATE data_acquisition_jobs SET quota_reserved=? WHERE organization_id=? AND actor_id=? AND id=?", row.QuotaReserved, row.OrganizationID, row.ActorID, row.ID).Error
}

func (r *Repository) Admit(ctx context.Context, p dataacquisition.Principal, command string, q dataacquisition.Query, funding orgresource.ResourceFunding) (dataacquisition.Job, error) {
	q, err := dataacquisition.NormalizeQuery(q)
	if err != nil || p.Scope.Validate() != nil || !collection.ValidID(command) || (p.CredentialID != "" && (!collection.ValidID(p.CredentialID) || p.CredentialRevision < 1)) || (funding != orgresource.FundingEnterprise && funding != orgresource.FundingMember) {
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	var out dataacquisition.Job
	finished := false
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		key, err := lockKey(tx, p.Scope, p.CredentialID)
		if err != nil {
			return err
		}
		checkCredential := func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := activeKey(tx, key, p.Scope); err != nil {
				return err
			}
			if p.CredentialID != "" && key.Revision != p.CredentialRevision {
				return dataacquisition.ErrConflict
			}
			return nil
		}
		if err = checkCredential(); err != nil {
			return err
		}
		readOriginal := func(row jobRow) error {
			if err := checkCredential(); err != nil {
				return err
			}
			out, err = summarize(tx, row)
			if err == nil {
				err = checkCredential()
			}
			finished = err == nil
			return err
		}
		// The locked canonical key supplies the effective bounded permission set.
		// Quota/revision changes do not alter an original command, permissions do.
		hash := collection.Digest(struct {
			Query       dataacquisition.Query
			Member, Key string
			Funding     orgresource.ResourceFunding
			Permissions []string
		}{q, p.Scope.MemberID, p.CredentialID, funding, key.Input.Permissions})
		// Original command replay never reserves quota or another job.
		var existing jobRow
		found := tx.Raw("SELECT "+jobColumns+" FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? AND command_key=?", p.Scope.OrganizationID, p.Scope.ActorID, command).Scan(&existing)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 1 {
			if existing.InputHash != hash {
				return dataacquisition.ErrConflict
			}
			return readOriginal(existing)
		}
		if err = r.live.CheckExecution(ctx, p, funding); err != nil {
			return err
		}
		var now time.Time
		if err = tx.Raw("SELECT now()").Scan(&now).Error; err != nil {
			return err
		}
		now = now.UTC()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		row := jobRow{OrganizationID: p.Scope.OrganizationID, ActorID: p.Scope.ActorID, MemberID: p.Scope.MemberID, ID: collection.StableID(p.Scope.OrganizationID, p.Scope.ActorID, "amazon-job", command), CommandKey: command, InputHash: hash, CredentialID: p.CredentialID, CredentialRevision: p.CredentialRevision, Funding: string(funding), State: "ADMITTED", QuotaReserved: q.Limit, DayWindow: day, MonthWindow: month, CreatedAt: now, Deadline: now.Add(30 * time.Minute)}
		if err = lockQuota(tx, row, true); err != nil {
			return err
		}
		if p.CredentialID != "" {
			var buckets []struct {
				WindowKind string
				Rows, Fen  int64
			}
			if err = tx.Raw("SELECT window_kind,consumed_rows+reserved_rows AS rows,consumed_fen+reserved_fen AS fen FROM data_service_quota WHERE organization_id=? AND actor_id=? AND key_id=? AND ((window_kind='day' AND window_start=?) OR (window_kind='month' AND window_start=?))", row.OrganizationID, row.ActorID, row.CredentialID, day, month).Scan(&buckets).Error; err != nil {
				return err
			}
			for _, b := range buckets {
				if b.WindowKind == "day" && b.Rows+int64(q.Limit) > key.Input.DailyRows || b.WindowKind == "month" && b.Fen+int64(q.Limit)*dataacquisition.PriceFen > key.Input.MonthlyCostFen {
					return dataacquisition.ErrConflict
				}
			}
		}
		if err = tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "amazon-jobs:"+row.OrganizationID).Error; err != nil {
			return err
		}
		// Console requests do not have a key lock. Recheck after the organization
		// admission lock to preserve the same command identity under concurrency.
		found = tx.Raw("SELECT "+jobColumns+" FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? AND command_key=?", p.Scope.OrganizationID, p.Scope.ActorID, command).Scan(&existing)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 1 {
			if existing.InputHash != hash {
				return dataacquisition.ErrConflict
			}
			return readOriginal(existing)
		}
		var active int64
		if err = tx.Raw("SELECT count(*) FROM data_acquisition_jobs WHERE organization_id=? AND state IN ('ADMITTED','RUNNING') AND NOT canceled AND deadline>now()", row.OrganizationID).Scan(&active).Error; err != nil {
			return err
		}
		if active >= 8 {
			return dataacquisition.ErrConflict
		}
		raw, err := json.Marshal(q)
		if err != nil {
			return err
		}
		var keyID, revision any
		if row.CredentialID != "" {
			keyID = row.CredentialID
			revision = row.CredentialRevision
		}
		if err = tx.Exec("INSERT INTO data_acquisition_jobs(organization_id,actor_id,member_id,id,command_key,input_hash,credential_id,credential_revision,query_json,funding,state,quota_reserved,day_window,month_window,created_at,deadline) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", row.OrganizationID, row.ActorID, row.MemberID, row.ID, command, hash, keyID, revision, string(raw), funding, row.State, row.QuotaReserved, day, month, now, row.Deadline).Error; err != nil {
			return err
		}
		if p.CredentialID != "" {
			for _, window := range []struct {
				kind  string
				start time.Time
			}{{"day", day}, {"month", month}} {
				if err = tx.Exec("UPDATE data_service_quota SET reserved_rows=reserved_rows+?,reserved_fen=reserved_fen+? WHERE organization_id=? AND actor_id=? AND key_id=? AND window_kind=? AND window_start=?", q.Limit, int64(q.Limit)*dataacquisition.PriceFen, row.OrganizationID, row.ActorID, row.CredentialID, window.kind, window.start).Error; err != nil {
					return err
				}
			}
		}
		row.QueryJSON = raw
		out, err = row.job()
		if err == nil {
			err = checkCredential()
		}
		finished = err == nil
		return err
	})
	if err != nil && finished {
		return dataacquisition.Job{}, dataacquisition.ErrUnknown
	}
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return out, err
}

// Every mutation follows key -> original UTC quota buckets -> job -> item.
// Proof/binding/fencing deliberately do not require continuing execution rights.
func (r *Repository) withJob(ctx context.Context, job dataacquisition.Job, active bool, action func(*gorm.DB, *jobRow, dataacquisition.Job) error) error {
	return r.withJobCommand(ctx, job, active, true, job.Scope, "", action)
}

func (r *Repository) withJobCommand(ctx context.Context, job dataacquisition.Job, active, checkLive bool, commandScope collection.Scope, command string, action func(*gorm.DB, *jobRow, dataacquisition.Job) error) error {
	if job.Scope.Validate() != nil || !collection.ValidID(job.ID) {
		return dataacquisition.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pre, err := readJob(r.db.WithContext(ctx), job.Scope, job.ID, false)
	if err != nil {
		return err
	}
	if pre.MemberID != job.Scope.MemberID || pre.InputHash != job.InputHash {
		return dataacquisition.ErrConflict
	}
	finished := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		apply := func(tx *gorm.DB) error {
			key, err := lockKey(tx, job.Scope, pre.CredentialID)
			if err != nil {
				return err
			}
			if err = lockQuota(tx, pre, false); err != nil {
				return err
			}
			row, err := readJob(tx, job.Scope, job.ID, true)
			if err != nil {
				return err
			}
			current, err := row.job()
			if err != nil {
				return err
			}
			if active {
				var now time.Time
				if err = tx.Raw("SELECT now()").Scan(&now).Error; err != nil {
					return err
				}
				if row.Canceled || !now.Before(row.Deadline) || (row.State != "ADMITTED" && row.State != "RUNNING") {
					return dataacquisition.ErrForbidden
				}
				if err = activeKey(tx, key, job.Scope); err != nil {
					return err
				}
				if checkLive {
					if err = r.live.CheckExecution(ctx, dataacquisition.Principal{Scope: current.Scope, CredentialID: current.CredentialID}, current.Funding); err != nil {
						return err
					}
				}
			}
			if err = action(tx, &row, current); err != nil {
				return err
			}
			return nil
		}
		var err error
		if command == "" {
			err = apply(tx)
		} else {
			err = keystore.ApplyCancellationCommand(tx, commandScope, command, job.ID, apply)
			if errors.Is(err, dataservice.ErrConflict) {
				err = dataacquisition.ErrConflict
			}
		}
		finished = err == nil
		return err
	})
	if err != nil && finished {
		return dataacquisition.ErrUnknown
	}
	return err
}
func (r *Repository) CheckActive(ctx context.Context, job dataacquisition.Job) error {
	return r.withJob(ctx, job, true, func(*gorm.DB, *jobRow, dataacquisition.Job) error { return nil })
}
func originalItemIntent(job dataacquisition.Job, asin string) orgresource.ConsumerChargeIntent {
	id := collection.StableID(job.Scope.OrganizationID, job.Scope.ActorID, "amazon-item", job.ID, asin)
	return orgresource.ConsumerChargeIntent{Identity: orgresource.ConsumerChargeIdentity{OrganizationID: job.Scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: id}, ActorID: job.Scope.ActorID, MemberID: job.Scope.MemberID, Funding: job.Funding, ResourceType: orgresource.ResourceDataRow, Quantity: 1, Fingerprint: collection.Digest(struct {
		Job, Query, ASIN string
		Price            int64
	}{job.ID, job.InputHash, asin, dataacquisition.PriceFen}), BusinessScope: "amazon-data:" + job.ID + ":" + id}
}
func (r *Repository) Discover(ctx context.Context, job dataacquisition.Job, ids []string) (dataacquisition.Job, error) {
	if len(ids) > 200 {
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	site, err := dataacquisition.ResolveSite(job.Query.Site)
	if err != nil {
		return dataacquisition.Job{}, err
	}
	normalized := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		asin, err := dataacquisition.NormalizeASIN(site, id)
		if err != nil || asin != id {
			return dataacquisition.Job{}, dataacquisition.ErrInvalid
		}
		if !seen[asin] {
			seen[asin] = true
			normalized = append(normalized, asin)
		}
	}
	if len(normalized) > job.Query.Limit {
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	// Empty discovery is also the terminal cleanup for cancellation/deadline.
	err = r.withJob(ctx, job, len(normalized) > 0, func(tx *gorm.DB, row *jobRow, current dataacquisition.Job) error {
		if row.Discovered {
			return nil
		}
		for _, asin := range normalized {
			intent := originalItemIntent(current, asin)
			id := intent.Identity.OperationID
			raw, err := json.Marshal(intent)
			if err != nil {
				return err
			}
			if err = tx.Exec("INSERT INTO data_acquisition_items(organization_id,actor_id,job_id,id,asin,state,intent_json,created_at) VALUES(?,?,?,?,?,'PREPARED',?,?)", row.OrganizationID, row.ActorID, row.ID, id, asin, string(raw), row.CreatedAt).Error; err != nil {
				return err
			}
		}
		if err := quotaMove(tx, row, row.QuotaReserved-len(normalized), 0); err != nil {
			return err
		}
		return tx.Exec("UPDATE data_acquisition_jobs SET discovered=true,state='RUNNING' WHERE organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Error
	})
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return committedJob(r.Read(ctx, job.Scope, job.ID))
}
func (r *Repository) Items(ctx context.Context, job dataacquisition.Job) ([]dataacquisition.Item, error) {
	if _, err := r.Read(ctx, job.Scope, job.ID); err != nil {
		return nil, err
	}
	rows, err := itemRows(r.db.WithContext(ctx), job)
	if err != nil {
		return nil, err
	}
	items := []dataacquisition.Item{}
	for _, row := range rows {
		i, err := row.item()
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (r *Repository) FailDiscovery(ctx context.Context, job dataacquisition.Job, reason string) (dataacquisition.Job, error) {
	switch reason {
	case "provider_rejected", "provider_challenged", "provider_unsupported", "deadline", "canceled", "access_revoked":
	default:
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	err := r.withJob(ctx, job, false, func(tx *gorm.DB, row *jobRow, _ dataacquisition.Job) error {
		if row.Discovered {
			return nil
		}
		if err := quotaMove(tx, row, row.QuotaReserved, 0); err != nil {
			return err
		}
		return tx.Exec("UPDATE data_acquisition_jobs SET discovered=true,state='FAILED',reason=? WHERE organization_id=? AND actor_id=? AND id=?", reason, row.OrganizationID, row.ActorID, row.ID).Error
	})
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return committedJob(r.Read(ctx, job.Scope, job.ID))
}
func (r *Repository) Claim(ctx context.Context, job dataacquisition.Job, id string) (dataacquisition.Item, error) {
	var item dataacquisition.Item
	err := r.withJob(ctx, job, true, func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, id, true)
		if err != nil {
			return err
		}
		var now time.Time
		if err = tx.Raw("SELECT now()").Scan(&now).Error; err != nil {
			return err
		}
		if row.ReservationID == "" || row.ChargeState != string(orgresource.ReservationReserved) || (row.State != "PREPARED" && row.State != "FETCHING") || (row.LeaseUntil != nil && now.Before(*row.LeaseUntil)) {
			return dataacquisition.ErrConflict
		}
		token := uuid.NewString()
		if err = tx.Exec("UPDATE data_acquisition_items SET state='FETCHING',claim_token=?,lease_until=? WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?", token, now.Add(30*time.Second), row.OrganizationID, row.ActorID, row.JobID, row.ID).Error; err != nil {
			return err
		}
		row.State = "FETCHING"
		row.ClaimToken = token
		item, err = row.item()
		return err
	})
	return item, err
}
func (r *Repository) BindReservation(ctx context.Context, job dataacquisition.Job, id string, receipt orgresource.ConsumerChargeReceipt) (dataacquisition.Item, error) {
	var item dataacquisition.Item
	err := r.withJob(ctx, job, false, func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, id, true)
		if err != nil {
			return err
		}
		if err = bind(tx, &row, receipt); err != nil {
			return err
		}
		item, err = row.item()
		return err
	})
	return item, err
}
func bind(tx *gorm.DB, row *itemRow, receipt orgresource.ConsumerChargeReceipt) error {
	intent, err := row.intent()
	if err != nil {
		return err
	}
	if intent != receipt.Intent || !collection.ValidID(receipt.ReservationID) || (row.ReservationID != "" && row.ReservationID != receipt.ReservationID) {
		return dataacquisition.ErrConflict
	}
	if receipt.State != orgresource.ReservationReserved && receipt.State != orgresource.ReservationCommitted && receipt.State != orgresource.ReservationReleased && receipt.State != orgresource.ReservationReconciliationRequired {
		return dataacquisition.ErrInvalid
	}
	if row.ChargeState == string(orgresource.ReservationCommitted) || row.ChargeState == string(orgresource.ReservationReleased) {
		if row.ChargeState != string(receipt.State) {
			return dataacquisition.ErrConflict
		}
		return nil
	}
	if receipt.State == orgresource.ReservationCommitted && row.State != "SAVED" || receipt.State == orgresource.ReservationReleased && row.State != "FAILED" {
		return dataacquisition.ErrUnknown
	}
	if err = tx.Exec("UPDATE data_acquisition_items SET reservation_id=?,charge_state=? WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?", receipt.ReservationID, receipt.State, row.OrganizationID, row.ActorID, row.JobID, row.ID).Error; err != nil {
		return err
	}
	row.ReservationID = receipt.ReservationID
	row.ChargeState = string(receipt.State)
	return nil
}
func (r *Repository) PrepareEvidence(ctx context.Context, job dataacquisition.Job, claimed dataacquisition.Item, e dataacquisition.Evidence) (dataacquisition.Item, error) {
	if e.Validate() != nil || e.Site != job.Query.Site || e.ASIN != claimed.ASIN {
		return dataacquisition.Item{}, dataacquisition.ErrInvalid
	}
	var item dataacquisition.Item
	err := r.withJob(ctx, job, true, func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, claimed.ID, true)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if row.State == "PREPARED_EVIDENCE" {
			if string(row.EvidenceJSON) != string(raw) {
				return dataacquisition.ErrConflict
			}
			item, err = row.item()
			return err
		}
		if row.State != "FETCHING" || claimed.ClaimToken == "" || row.ClaimToken != claimed.ClaimToken {
			return dataacquisition.ErrConflict
		}
		if err = tx.Exec("UPDATE data_acquisition_items SET evidence_json=?,state='PREPARED_EVIDENCE',lease_until=NULL WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?", raw, row.OrganizationID, row.ActorID, row.JobID, row.ID).Error; err != nil {
			return err
		}
		row.State = "PREPARED_EVIDENCE"
		row.EvidenceJSON = raw
		item, err = row.item()
		return err
	})
	return item, err
}
func (r *Repository) Publish(ctx context.Context, job dataacquisition.Job, prepared dataacquisition.Item) (dataacquisition.Item, error) {
	var item dataacquisition.Item
	// Saved replay is a lookup of the exact previous fact, including after revoke.
	existing, err := readItem(r.db.WithContext(ctx), job, prepared.ID, false)
	if err != nil {
		return item, err
	}
	if existing.State == "SAVED" {
		return existing.item()
	}
	err = r.withJob(ctx, job, true, func(tx *gorm.DB, row *jobRow, current dataacquisition.Job) error {
		locked, err := readItem(tx, current, prepared.ID, true)
		if err != nil {
			return err
		}
		if locked.State == "SAVED" {
			item, err = locked.item()
			return err
		}
		if locked.State != "PREPARED_EVIDENCE" || locked.ReservationID == "" || locked.ChargeState != string(orgresource.ReservationReserved) {
			return dataacquisition.ErrConflict
		}
		fixed, err := locked.item()
		if err != nil || fixed.Evidence == nil {
			return dataacquisition.ErrUnavailable
		}
		source, err := r.publish(ctx, tx, current, fixed)
		if err != nil {
			return err
		}
		if source.Kind != "amazon_data" || source.OperationID != fixed.ID || source.PublicationID != fixed.ID || source.Version == 0 {
			return dataacquisition.ErrConflict
		}
		batch, err := collection.NewPublicationBatch(current.Scope, current.ID, "amazon_data", "Amazon · "+current.Query.Site)
		if err != nil {
			return err
		}
		if _, err = collectionstore.AppendDataPublication(ctx, tx, current.Scope, batch, source, time.Now().UTC()); err != nil {
			return err
		}
		if err = quotaMove(tx, row, 1, 1); err != nil {
			return err
		}
		raw, err := json.Marshal(source)
		if err != nil {
			return err
		}
		if err = tx.Exec("UPDATE data_acquisition_items SET source_json=?,state='SAVED',saved_at=now(),terminal_evidence=?,claim_token=NULL,lease_until=NULL WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?", string(raw), uuid.NewString(), locked.OrganizationID, locked.ActorID, locked.JobID, locked.ID).Error; err != nil {
			return err
		}
		locked.SourceJSON = raw
		locked.State = "SAVED"
		item, err = locked.item()
		return err
	})
	return item, err
}
func failItem(tx *gorm.DB, job *jobRow, row *itemRow, reason string) error {
	if row.State == "SAVED" || row.State == "FAILED" {
		return nil
	}
	if err := quotaMove(tx, job, 1, 0); err != nil {
		return err
	}
	row.State = "FAILED"
	row.Reason = reason
	row.TerminalEvidence = uuid.NewString()
	row.ClaimToken = ""
	return tx.Exec("UPDATE data_acquisition_items SET state='FAILED',reason=?,terminal_evidence=?,claim_token=NULL,lease_until=NULL WHERE organization_id=? AND actor_id=? AND job_id=? AND id=?", reason, row.TerminalEvidence, row.OrganizationID, row.ActorID, row.JobID, row.ID).Error
}
func (r *Repository) Fence(ctx context.Context, job dataacquisition.Job, claimed dataacquisition.Item, reason string) (dataacquisition.Item, error) {
	providerFailure := false
	switch reason {
	case "provider_rejected", "provider_challenged", "provider_unsupported":
		providerFailure = true
	case "canceled", "deadline", "access_revoked", "resource_unavailable":
	default:
		return dataacquisition.Item{}, dataacquisition.ErrInvalid
	}
	var item dataacquisition.Item
	err := r.withJob(ctx, job, false, func(tx *gorm.DB, j *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, claimed.ID, true)
		if err != nil {
			return err
		}
		if providerFailure && row.State != "SAVED" && row.State != "FAILED" && (row.State != "FETCHING" || claimed.ClaimToken == "" || row.ClaimToken != claimed.ClaimToken) {
			return dataacquisition.ErrConflict
		}
		if err = failItem(tx, j, &row, reason); err != nil {
			return err
		}
		item, err = row.item()
		return err
	})
	return item, err
}
func (r *Repository) RecordCharge(ctx context.Context, job dataacquisition.Job, id string, receipt orgresource.ConsumerChargeReceipt) error {
	return r.withJob(ctx, job, false, func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, id, true)
		if err != nil {
			return err
		}
		if receipt.State == orgresource.ReservationCommitted || receipt.State == orgresource.ReservationReleased {
			if receipt.OwnerEvidenceID != row.TerminalEvidence {
				return dataacquisition.ErrConflict
			}
		}
		return bind(tx, &row, receipt)
	})
}
func (r *Repository) Finish(ctx context.Context, job dataacquisition.Job) (dataacquisition.Job, error) {
	err := r.withJob(ctx, job, false, func(tx *gorm.DB, row *jobRow, current dataacquisition.Job) error {
		summary, err := summarize(tx, *row)
		if err != nil {
			return err
		}
		if !row.Discovered || summary.Pending > 0 {
			return nil
		}
		state := "SUCCEEDED"
		if row.Canceled {
			state = "CANCELED"
		} else if row.Reason != "" && summary.Saved == 0 {
			state = "FAILED"
		} else if summary.Failed > 0 {
			state = "PARTIAL"
			if summary.Saved == 0 {
				state = "FAILED"
			}
		}
		return tx.Exec("UPDATE data_acquisition_jobs SET state=? WHERE organization_id=? AND actor_id=? AND id=?", state, row.OrganizationID, row.ActorID, row.ID).Error
	})
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return committedJob(r.Read(ctx, job.Scope, job.ID))
}
func (r *Repository) Cancel(ctx context.Context, s collection.Scope, id, command string) (dataacquisition.Job, error) {
	if !collection.ValidID(command) {
		return dataacquisition.Job{}, dataacquisition.ErrInvalid
	}
	job, err := r.Read(ctx, s, id)
	if err != nil {
		return job, err
	}
	err = r.withJobCommand(ctx, job, false, true, s, command, func(tx *gorm.DB, row *jobRow, current dataacquisition.Job) error {
		if row.Canceled {
			return nil
		}
		if row.State != "ADMITTED" && row.State != "RUNNING" {
			return dataacquisition.ErrConflict
		}
		rows, err := itemRows(tx, current)
		if err != nil {
			return err
		}
		for _, i := range rows {
			locked, err := readItem(tx, current, i.ID, true)
			if err != nil {
				return err
			}
			if err = failItem(tx, row, &locked, "canceled"); err != nil {
				return err
			}
		}
		if row.QuotaReserved > 0 {
			if err = quotaMove(tx, row, row.QuotaReserved, 0); err != nil {
				return err
			}
		}
		return tx.Exec("UPDATE data_acquisition_jobs SET canceled=true,discovered=true,state='CANCELED' WHERE organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Error
	})
	if err != nil {
		return dataacquisition.Job{}, err
	}
	return committedJob(r.Read(ctx, s, id))
}
func (r *Repository) original(ctx context.Context, id orgresource.ConsumerChargeIdentity) (dataacquisition.Job, itemRow, error) {
	if !orgresource.ValidConsumerChargeIdentity(id) || id.Consumer != orgresource.ConsumerAmazonData || !collection.ValidID(id.OperationID) {
		return dataacquisition.Job{}, itemRow{}, dataacquisition.ErrInvalid
	}
	var row itemRow
	result := r.db.WithContext(ctx).Raw("SELECT "+itemColumns+" FROM data_acquisition_items WHERE organization_id=? AND id=?", id.OrganizationID, id.OperationID).Scan(&row)
	if result.Error != nil {
		return dataacquisition.Job{}, row, result.Error
	}
	if result.RowsAffected != 1 {
		return dataacquisition.Job{}, row, dataacquisition.ErrNotFound
	}
	var member string
	result = r.db.WithContext(ctx).Raw("SELECT member_id FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.JobID).Scan(&member)
	if result.Error != nil {
		return dataacquisition.Job{}, row, result.Error
	}
	if result.RowsAffected != 1 {
		return dataacquisition.Job{}, row, dataacquisition.ErrUnavailable
	}
	job, err := r.Read(ctx, collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: member}, row.JobID)
	return job, row, err
}
func (r *Repository) ChargeIntent(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	job, _, err := r.original(ctx, id)
	if err != nil {
		return orgresource.ConsumerChargeIntent{}, err
	}
	// ProcessItem checks live access before every new Reserve and again before
	// Fetch. Resource's 500ms owner callback reads only locked original facts.
	var intent orgresource.ConsumerChargeIntent
	err = r.withJobCommand(ctx, job, true, false, job.Scope, "", func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, id.OperationID, true)
		if err != nil {
			return err
		}
		if row.State == "SAVED" || row.State == "FAILED" {
			return dataacquisition.ErrForbidden
		}
		candidate, err := row.intent()
		if err != nil {
			return err
		}
		if candidate != originalItemIntent(current, row.ASIN) || candidate.Identity != id {
			return dataacquisition.ErrConflict
		}
		intent = candidate
		return nil
	})
	if err != nil {
		return orgresource.ConsumerChargeIntent{}, err
	}
	return intent, nil
}
func (r *Repository) ChargeProof(ctx context.Context, receipt orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	job, _, err := r.original(ctx, receipt.Intent.Identity)
	if err != nil {
		return orgresource.ConsumerChargeProof{}, err
	}
	proof := orgresource.ConsumerChargeProof{Intent: receipt.Intent, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectUnknown}
	err = r.withJob(ctx, job, false, func(tx *gorm.DB, _ *jobRow, current dataacquisition.Job) error {
		row, err := readItem(tx, current, receipt.Intent.Identity.OperationID, true)
		if err != nil {
			return err
		}
		if err = bind(tx, &row, receipt); err != nil {
			return err
		}
		if row.State == "SAVED" {
			proof.State = orgresource.ConsumerEffectSucceeded
		} else if row.State == "FAILED" {
			proof.State = orgresource.ConsumerEffectFailed
		}
		if proof.State != orgresource.ConsumerEffectUnknown {
			proof.EvidenceID = row.TerminalEvidence
		}
		return nil
	})
	return proof, err
}

var _ dataacquisition.Repository = (*Repository)(nil)

func committedJob(job dataacquisition.Job, err error) (dataacquisition.Job, error) {
	if err != nil {
		return dataacquisition.Job{}, dataacquisition.ErrUnknown
	}
	return job, nil
}
