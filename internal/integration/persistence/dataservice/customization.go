package dataservicepersistence

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"strconv"
	"task-processor/internal/authidentity"
	"task-processor/internal/dataservice"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/product/collection"
	"time"
)

type CustomPublisher func(context.Context, *gorm.DB, dataservice.DeliveryAuthority, string, collection.OwnProduct) (collection.Source, error)
type CustomRepository struct {
	db      *gorm.DB
	special dataservice.SpecialistAccess
	publish CustomPublisher
}

func NewCustomRepository(ctx context.Context, db *gorm.DB, special dataservice.SpecialistAccess, publish CustomPublisher) (*CustomRepository, error) {
	if special == nil || publish == nil {
		return nil, dataservice.ErrUnavailable
	}
	if err := VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	if err := VerifyCustomSchema(ctx, db); err != nil {
		return nil, err
	}
	if err := collectionstore.VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	return &CustomRepository{db, special, publish}, nil
}

type customRow struct {
	OrganizationID, ActorID, MemberID, ID, State, BatchID string
	InputJSON, SpecJSON                                   []byte
	Revision, SpecRevision                                int64
	DeliveredRows                                         int
	CreatedAt                                             time.Time
}

const customColumns = "organization_id,actor_id,member_id,id,input_json,state,revision,spec_json,spec_revision,COALESCE(batch_id::text,'') AS batch_id,delivered_rows,created_at"

func (row customRow) request() (dataservice.CustomRequest, error) {
	var input dataservice.CustomInput
	if json.Unmarshal(row.InputJSON, &input) != nil {
		return dataservice.CustomRequest{}, dataservice.ErrUnavailable
	}
	request := dataservice.CustomRequest{ID: row.ID, Scope: collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}, Input: input, State: row.State, Revision: row.Revision, SpecRevision: row.SpecRevision, BatchID: row.BatchID, DeliveredRows: row.DeliveredRows, CreatedAt: row.CreatedAt, Events: []dataservice.CustomEvent{}}
	if len(row.SpecJSON) > 0 {
		var spec dataservice.CustomSpec
		if json.Unmarshal(row.SpecJSON, &spec) != nil {
			return request, dataservice.ErrUnavailable
		}
		request.Spec = &spec
	}
	return request, nil
}
func readCustom(db *gorm.DB, id string, scope *collection.Scope, lock bool) (customRow, error) {
	if !collection.ValidID(id) {
		return customRow{}, dataservice.ErrInvalid
	}
	query := "SELECT " + customColumns + " FROM data_service_custom_requests WHERE id=?"
	args := []any{id}
	if scope != nil {
		query += " AND organization_id=? AND actor_id=?"
		args = append(args, scope.OrganizationID, scope.ActorID)
	}
	if lock {
		query += " FOR UPDATE"
	}
	var row customRow
	result := db.Raw(query, args...).Scan(&row)
	if result.Error != nil {
		return row, result.Error
	}
	if result.RowsAffected != 1 {
		return row, dataservice.ErrNotFound
	}
	return row, nil
}
func customDetails(db *gorm.DB, row customRow) (dataservice.CustomRequest, error) {
	request, err := row.request()
	if err != nil {
		return request, err
	}
	var events []struct {
		Revision                int64
		State, OperatorID, Note string
		CreatedAt               time.Time
	}
	if err = db.Raw("SELECT revision,state,operator_id,note,created_at FROM data_service_custom_events WHERE organization_id=? AND actor_id=? AND request_id=? ORDER BY revision DESC LIMIT 101", row.OrganizationID, row.ActorID, row.ID).Scan(&events).Error; err != nil {
		return request, err
	}
	if len(events) > 100 {
		events = events[:100]
		request.NextEventBefore = events[99].Revision
	}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		request.Events = append(request.Events, dataservice.CustomEvent{Revision: e.Revision, State: e.State, OperatorID: e.OperatorID, Note: e.Note, At: e.CreatedAt})
	}
	return request, nil
}
func customEvent(tx *gorm.DB, request dataservice.CustomRequest, operator, note string) error {
	return tx.Exec("INSERT INTO data_service_custom_events(organization_id,actor_id,request_id,revision,state,operator_id,note,created_at) VALUES(?,?,?,?,?,?,?,?)", request.Scope.OrganizationID, request.Scope.ActorID, request.ID, request.Revision, request.State, operator, note, time.Now().UTC()).Error
}
func (r *CustomRepository) Read(ctx context.Context, s collection.Scope, id string) (dataservice.CustomRequest, error) {
	if s.Validate() != nil {
		return dataservice.CustomRequest{}, dataservice.ErrForbidden
	}
	row, err := readCustom(r.db.WithContext(ctx), id, &s, false)
	if err != nil {
		return dataservice.CustomRequest{}, err
	}
	return customDetails(r.db.WithContext(ctx), row)
}
func (r *CustomRepository) List(ctx context.Context, s collection.Scope, limit int) ([]dataservice.CustomRequest, error) {
	if s.Validate() != nil || limit < 1 || limit > 100 {
		return nil, dataservice.ErrInvalid
	}
	var rows []customRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+customColumns+" FROM data_service_custom_requests WHERE organization_id=? AND actor_id=? ORDER BY created_at DESC,id LIMIT ?", s.OrganizationID, s.ActorID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := []dataservice.CustomRequest{}
	for _, row := range rows {
		request, err := row.request()
		if err != nil {
			return nil, err
		}
		out = append(out, request)
	}
	return out, nil
}
func (r *CustomRepository) operator(ctx context.Context, expected dataservice.Operator) error {
	if !authidentity.IsBoundedIdentifier(expected.ID) {
		return dataservice.ErrForbidden
	}
	current, err := r.special.Specialist(ctx)
	if err != nil {
		return err
	}
	if current != expected {
		return dataservice.ErrForbidden
	}
	return nil
}
func (r *CustomRepository) AdminRead(ctx context.Context, op dataservice.Operator, id string) (dataservice.CustomRequest, error) {
	if err := r.operator(ctx, op); err != nil {
		return dataservice.CustomRequest{}, err
	}
	row, err := readCustom(r.db.WithContext(ctx), id, nil, false)
	if err != nil {
		return dataservice.CustomRequest{}, err
	}
	return customDetails(r.db.WithContext(ctx), row)
}
func customState(state string) bool {
	switch state {
	case "", "SUBMITTED", "EVALUATING", "SPEC_CONFIRMED", "PREPARING", "DELIVERED", "CLOSED":
		return true
	}
	return false
}
func (r *CustomRepository) AdminList(ctx context.Context, op dataservice.Operator, state, cursor string, limit int) (dataservice.CustomPage, error) {
	if err := r.operator(ctx, op); err != nil {
		return dataservice.CustomPage{}, err
	}
	if !customState(state) || (cursor != "" && !collection.ValidID(cursor)) || limit < 1 || limit > 100 {
		return dataservice.CustomPage{}, dataservice.ErrInvalid
	}
	query := "SELECT " + customColumns + " FROM data_service_custom_requests WHERE true"
	args := []any{}
	if state != "" {
		query += " AND state=?"
		args = append(args, state)
	}
	if cursor != "" {
		query += " AND id>?"
		args = append(args, cursor)
	}
	query += " ORDER BY id LIMIT ?"
	args = append(args, limit+1)
	var rows []customRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return dataservice.CustomPage{}, err
	}
	page := dataservice.CustomPage{Items: []dataservice.CustomRequest{}}
	if len(rows) > limit {
		page.NextCursor = rows[limit-1].ID
		rows = rows[:limit]
	}
	for _, row := range rows {
		request, err := row.request()
		if err != nil {
			return dataservice.CustomPage{}, err
		}
		page.Items = append(page.Items, request)
	}
	return page, nil
}
func (r *CustomRepository) Submit(ctx context.Context, s collection.Scope, command string, input dataservice.CustomInput) (dataservice.CustomRequest, error) {
	input, err := dataservice.NormalizeCustomInput(input)
	if err != nil || s.Validate() != nil || !collection.ValidID(command) {
		return dataservice.CustomRequest{}, dataservice.ErrInvalid
	}
	hash := collection.Digest(input)
	id := collection.StableID(s.OrganizationID, s.ActorID, "custom-request", command)
	finished := false
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := actorLock(tx, s); err != nil {
			return err
		}
		old, replay, err := readCommand(tx, s, command, hash, "submit_custom")
		if err != nil {
			return err
		}
		if replay {
			id = old.TargetID
			finished = true
			return nil
		}
		raw, err := json.Marshal(input)
		if err != nil || len(raw) > 65536 {
			return dataservice.ErrInvalid
		}
		if err = tx.Exec("INSERT INTO data_service_custom_requests(organization_id,actor_id,member_id,id,input_json,state,revision,created_at) VALUES(?,?,?,?,?,'SUBMITTED',1,?)", s.OrganizationID, s.ActorID, s.MemberID, id, string(raw), time.Now().UTC()).Error; err != nil {
			return err
		}
		request := dataservice.CustomRequest{ID: id, Scope: s, State: "SUBMITTED", Revision: 1}
		if err = customEvent(tx, request, s.ActorID, "提交需求"); err != nil {
			return err
		}
		if err = writeCommand(tx, s, command, hash, "submit_custom", id, map[string]string{"requestId": id}); err != nil {
			return err
		}
		finished = true
		return nil
	})
	if err != nil {
		if finished {
			return dataservice.CustomRequest{}, dataservice.ErrUnknown
		}
		return dataservice.CustomRequest{}, err
	}
	return r.Read(ctx, s, id)
}

type customCommand struct {
	RequestID, InputHash, Action string
	Revision                     int64
}

func (r *CustomRepository) Command(ctx context.Context, op dataservice.Operator, command string) (dataservice.CustomRequest, error) {
	if !collection.ValidID(command) {
		return dataservice.CustomRequest{}, dataservice.ErrInvalid
	}
	current, err := r.special.Specialist(ctx)
	if err != nil {
		return dataservice.CustomRequest{}, err
	}
	if current != op {
		return dataservice.CustomRequest{}, dataservice.ErrForbidden
	}
	var id string
	found := r.db.WithContext(ctx).Raw("SELECT request_id FROM data_service_custom_commands WHERE operator_id=? AND command_key=?", op.ID, command).Scan(&id)
	if found.Error != nil {
		return dataservice.CustomRequest{}, found.Error
	}
	if found.RowsAffected != 1 {
		return dataservice.CustomRequest{}, dataservice.ErrNotFound
	}
	return r.AdminRead(ctx, op, id)
}

func readCustomCommand(tx *gorm.DB, op dataservice.Operator, id, command, hash, action string) (bool, error) {
	var old customCommand
	found := tx.Raw("SELECT request_id,input_hash,action,revision FROM data_service_custom_commands WHERE operator_id=? AND command_key=?", op.ID, command).Scan(&old)
	if found.Error != nil {
		return false, found.Error
	}
	if found.RowsAffected == 0 {
		return false, nil
	}
	if old.RequestID != id || old.InputHash != hash || old.Action != action {
		return false, dataservice.ErrConflict
	}
	return true, nil
}
func writeCustomCommand(tx *gorm.DB, op dataservice.Operator, id, command, hash, action string, revision int64) error {
	return tx.Exec("INSERT INTO data_service_custom_commands(operator_id,command_key,request_id,input_hash,action,revision,created_at) VALUES(?,?,?,?,?,?,?)", op.ID, command, id, hash, action, revision, time.Now().UTC()).Error
}
func operatorLock(tx *gorm.DB, op dataservice.Operator) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "data-specialist:"+op.ID).Error
}
func (r *CustomRepository) Change(ctx context.Context, op dataservice.Operator, id, command string, revision int64, patch dataservice.CustomPatch) (dataservice.CustomRequest, error) {
	if !collection.ValidID(id) || !collection.ValidID(command) || revision < 1 {
		return dataservice.CustomRequest{}, dataservice.ErrInvalid
	}
	if err := r.operator(ctx, op); err != nil {
		return dataservice.CustomRequest{}, err
	}
	hash := collection.Digest(struct {
		ID       string
		Revision int64
		Patch    dataservice.CustomPatch
	}{id, revision, patch})
	finished := false
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := operatorLock(tx, op); err != nil {
			return err
		}
		replay, err := readCustomCommand(tx, op, id, command, hash, "change")
		if err != nil {
			return err
		}
		if replay {
			finished = true
			return nil
		}
		row, err := readCustom(tx, id, nil, true)
		if err != nil {
			return err
		}
		request, err := row.request()
		if err != nil {
			return err
		}
		if request.Revision != revision {
			return dataservice.ErrConflict
		}
		if err = dataservice.ValidateCustomChange(request, patch); err != nil {
			return err
		}
		if err = r.operator(ctx, op); err != nil {
			return err
		}
		request.State = patch.State
		request.Revision++
		var spec any
		if patch.Spec != nil {
			request.Spec = patch.Spec
			request.SpecRevision = request.Revision
			raw, err := json.Marshal(patch.Spec)
			if err != nil {
				return err
			}
			spec = string(raw)
		} else if len(row.SpecJSON) > 0 {
			spec = string(row.SpecJSON)
		}
		if err = tx.Exec("UPDATE data_service_custom_requests SET state=?,revision=?,spec_json=?,spec_revision=? WHERE id=?", request.State, request.Revision, spec, request.SpecRevision, id).Error; err != nil {
			return err
		}
		if err = customEvent(tx, request, op.ID, patch.Note); err != nil {
			return err
		}
		if err = writeCustomCommand(tx, op, id, command, hash, "change", request.Revision); err != nil {
			return err
		}
		finished = true
		return nil
	})
	if err != nil {
		if finished {
			return dataservice.CustomRequest{}, dataservice.ErrUnknown
		}
		return dataservice.CustomRequest{}, err
	}
	return r.AdminRead(ctx, op, id)
}
func (r *CustomRepository) Deliver(ctx context.Context, a dataservice.DeliveryAuthority, command string, products []collection.OwnProduct) (dataservice.CustomRequest, error) {
	if !a.Valid() || !collection.ValidID(command) || len(products) < 1 || len(products) > 200 {
		return dataservice.CustomRequest{}, dataservice.ErrInvalid
	}
	if err := r.operator(ctx, a.Operator()); err != nil {
		return dataservice.CustomRequest{}, err
	}
	raw, err := json.Marshal(products)
	if err != nil || len(raw) > collection.MaxPayloadBytes {
		return dataservice.CustomRequest{}, dataservice.ErrInvalid
	}
	hash := collection.Digest(struct {
		ID             string
		Products       json.RawMessage
		RevisionMarker string
	}{a.RequestID(), raw, a.Fingerprint()})
	finished := false
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := operatorLock(tx, a.Operator()); err != nil {
			return err
		}
		replay, err := readCustomCommand(tx, a.Operator(), a.RequestID(), command, hash, "deliver")
		if err != nil {
			return err
		}
		if replay {
			finished = true
			return nil
		}
		scope := a.Scope()
		row, err := readCustom(tx, a.RequestID(), &scope, true)
		if err != nil {
			return err
		}
		request, err := row.request()
		if err != nil {
			return err
		}
		if !a.Matches(request) || len(products) > request.Spec.MaximumRows {
			return dataservice.ErrConflict
		}
		if err = r.operator(ctx, a.Operator()); err != nil {
			return err
		}
		if err = r.special.CheckApplicant(ctx, request.Scope); err != nil {
			return err
		}
		batch, err := collection.NewPublicationBatch(request.Scope, collection.StableID(a.RequestID(), command), "custom_dataset", request.Input.Name)
		if err != nil {
			return err
		}
		for i, product := range products {
			operation := collection.StableID(request.Scope.OrganizationID, request.Scope.ActorID, "custom-row", a.RequestID(), command, strconv.Itoa(i))
			source, err := r.publish(ctx, tx, a, operation, product)
			if err != nil {
				return err
			}
			if source.OperationID != operation || source.PublicationID != operation || source.Kind != "custom_dataset" || source.Version == 0 {
				return dataservice.ErrConflict
			}
			if _, err = collectionstore.AppendDataPublication(ctx, tx, request.Scope, batch, source, time.Now().UTC()); err != nil {
				return err
			}
		}
		if err = r.special.CheckApplicant(ctx, request.Scope); err != nil {
			return err
		}
		request.State = "DELIVERED"
		request.Revision++
		request.BatchID = batch.ID
		request.DeliveredRows = len(products)
		if err = tx.Exec("UPDATE data_service_custom_requests SET state='DELIVERED',revision=?,batch_id=?,delivered_rows=? WHERE id=?", request.Revision, batch.ID, len(products), request.ID).Error; err != nil {
			return err
		}
		if err = customEvent(tx, request, a.Operator().ID, "交付 "+strconv.Itoa(len(products))+" 条专员声明数据"); err != nil {
			return err
		}
		if err = writeCustomCommand(tx, a.Operator(), request.ID, command, hash, "deliver", request.Revision); err != nil {
			return err
		}
		finished = true
		return nil
	})
	if err != nil {
		if finished {
			return dataservice.CustomRequest{}, dataservice.ErrUnknown
		}
		return dataservice.CustomRequest{}, err
	}
	return r.AdminRead(ctx, a.Operator(), a.RequestID())
}

// The private authority includes the frozen request/spec revisions in its
// digest, despite those fields deliberately being absent from JSON.

var _ dataservice.CustomRepository = (*CustomRepository)(nil)
