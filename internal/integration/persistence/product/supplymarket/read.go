package supplymarketpersistence

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"
)

type recordRow struct {
	ID, OrganizationID, ActorID, MemberID, Kind, Stage string
	Revision                                           int64
	RecordJSON                                         []byte
}

const recordColumns = "id,organization_id,actor_id,member_id,kind,stage,revision,record_json"

var lockUpdate = clause.Locking{Strength: "UPDATE"}

func (row recordRow) record() (supplymarket.Record, error) {
	var record supplymarket.Record
	if len(row.RecordJSON) > 2<<20 || json.Unmarshal(row.RecordJSON, &record) != nil || record.ID != row.ID || record.Revision != row.Revision || record.Kind != row.Kind || string(record.Stage) != row.Stage {
		return record, supplymarket.ErrUnavailable
	}
	record.Owner = collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}
	if record.Owner.Validate() != nil {
		return supplymarket.Record{}, supplymarket.ErrUnavailable
	}
	if record.Source != nil {
		record.Source.Scope = record.Owner
	}
	return record, nil
}
func readRecord(db *gorm.DB, scope collection.Scope, platform, id string, lock bool) (supplymarket.Record, error) {
	if _, _, _, err := principal(scope, platform); err != nil {
		return supplymarket.Record{}, err
	}
	if !collection.ValidID(id) {
		return supplymarket.Record{}, supplymarket.ErrInvalid
	}
	query := db.Table("supply_market_records").Where("id=?", id)
	if platform == "" {
		query = query.Where("organization_id=? AND actor_id=? AND member_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID)
	}
	var row recordRow
	if lock {
		query = query.Clauses(lockUpdate)
	}
	result := query.Select(recordColumns).Take(&row)
	if result.Error == gorm.ErrRecordNotFound {
		return supplymarket.Record{}, supplymarket.ErrNotFound
	}
	if result.Error != nil {
		return supplymarket.Record{}, result.Error
	}
	return row.record()
}

type releaseRow struct {
	ID, RecordID, OrganizationID, ActorID, MemberID, Channel string
	Revision                                                 int64
	Active                                                   bool
	ReleaseJSON, SourceJSON                                  []byte
}

const releaseColumns = "id,record_id,organization_id,actor_id,member_id,channel,revision,active,release_json,source_json"

func (row releaseRow) release() (supplymarket.Release, error) {
	var release supplymarket.Release
	if len(row.ReleaseJSON) > 2<<20 || len(row.SourceJSON) > 64<<10 || json.Unmarshal(row.ReleaseJSON, &release) != nil || json.Unmarshal(row.SourceJSON, &release.Source) != nil || release.ID != row.ID || release.Channel != row.Channel || release.Revision != row.Revision || release.Active != row.Active {
		return release, supplymarket.ErrUnavailable
	}
	release.RecordID = row.RecordID
	release.OriginalOwner = collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}
	if release.OriginalOwner.Validate() != nil {
		return supplymarket.Release{}, supplymarket.ErrUnavailable
	}
	release.Source.Scope = release.OriginalOwner
	return release, nil
}
func readRelease(db *gorm.DB, id string, lock bool) (supplymarket.Release, error) {
	if !collection.ValidID(id) {
		return supplymarket.Release{}, supplymarket.ErrInvalid
	}
	query := db.Table("supply_market_releases").Where("id=? AND active", id)
	if lock {
		query = query.Clauses(lockUpdate)
	}
	var row releaseRow
	result := query.Select(releaseColumns).Take(&row)
	if result.Error == gorm.ErrRecordNotFound {
		return supplymarket.Release{}, supplymarket.ErrNotFound
	}
	if result.Error != nil {
		return supplymarket.Release{}, result.Error
	}
	return row.release()
}
func (r *Repository) ReadRecord(ctx context.Context, scope collection.Scope, platform, id string) (supplymarket.Record, error) {
	return readRecord(r.db.WithContext(ctx), scope, platform, id, false)
}
func (r *Repository) ReadRelease(ctx context.Context, id string) (supplymarket.Release, error) {
	return readRelease(r.db.WithContext(ctx), id, false)
}
func (r *Repository) ListRecordReleases(ctx context.Context, scope collection.Scope, platform, id string, q supplymarket.Query) (supplymarket.Page[supplymarket.Release], error) {
	var page supplymarket.Page[supplymarket.Release]
	if q.Validate() != nil {
		return page, supplymarket.ErrInvalid
	}
	db := r.db.WithContext(ctx)
	if _, err := readRecord(db, scope, platform, id, false); err != nil {
		return page, err
	}
	base := db.Table("supply_market_releases").Where("record_id=?", id)
	if err := base.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if q.After != "" {
		base = base.Where("id>?", q.After)
	}
	var rows []releaseRow
	columns := strings.Replace(releaseColumns, "release_json", `jsonb_set(release_json,'{product}',jsonb_build_object('title',release_json->'product'->'title','images',jsonb_build_array(release_json->'product'->'images'->0))) AS release_json`, 1)
	if err := base.Select(columns).Order("id").Limit(q.Limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		release, err := row.release()
		if err != nil {
			return supplymarket.Page[supplymarket.Release]{}, err
		}
		page.Items = append(page.Items, release)
	}
	return page, ctx.Err()
}
func (r *Repository) ReadOperation(ctx context.Context, scope collection.Scope, platform, id string) (supplymarket.Receipt, error) {
	p, actor, member, err := principal(scope, platform)
	if err != nil {
		return supplymarket.Receipt{}, err
	}
	if !collection.ValidID(id) {
		return supplymarket.Receipt{}, supplymarket.ErrInvalid
	}
	var row struct{ ReceiptJSON []byte }
	result := r.db.WithContext(ctx).Raw("SELECT receipt_json FROM supply_market_commands WHERE principal=? AND actor_id=? AND member_id=? AND operation_id=?", p, actor, member, id).Scan(&row)
	if result.Error != nil {
		return supplymarket.Receipt{}, result.Error
	}
	if result.RowsAffected != 1 {
		return supplymarket.Receipt{}, supplymarket.ErrNotFound
	}
	var receipt supplymarket.Receipt
	if len(row.ReceiptJSON) > 64<<10 || json.Unmarshal(row.ReceiptJSON, &receipt) != nil || receipt.OperationID != id {
		return receipt, supplymarket.ErrUnavailable
	}
	receipt.Replayed = true
	return receipt, nil
}
func keyword(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value) + "%"
}
func (r *Repository) ListRecords(ctx context.Context, scope collection.Scope, platform string, q supplymarket.Query) (supplymarket.Page[supplymarket.Record], error) {
	page := supplymarket.Page[supplymarket.Record]{Items: []supplymarket.Record{}}
	if q.Validate() != nil {
		return page, supplymarket.ErrInvalid
	}
	if _, _, _, err := principal(scope, platform); err != nil {
		return page, err
	}
	query := r.db.WithContext(ctx).Table("supply_market_records")
	if platform == "" {
		query = query.Where("organization_id=? AND actor_id=? AND member_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID)
	}
	if q.Kind != "" {
		query = query.Where("kind=?", q.Kind)
	}
	if q.Ended {
		query = query.Where("stage IN ('APPROVED','REJECTED','PLAN_CONFIRMED','CLOSED')")
	} else {
		query = query.Where("stage NOT IN ('APPROVED','REJECTED','PLAN_CONFIRMED','CLOSED')")
	}
	if q.Keyword != "" {
		query = query.Where("COALESCE(record_json->'product'->>'title',record_json->'connection'->>'name','') ILIKE ?", keyword(q.Keyword))
	}
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if q.After != "" {
		query = query.Where("id>?::uuid", q.After)
	}
	var rows []recordRow
	columns := strings.Replace(recordColumns, "record_json", `CASE WHEN record_json->'product' IS NULL THEN record_json ELSE jsonb_set(record_json,'{product}',jsonb_build_object('title',record_json->'product'->'title','images',jsonb_build_array(record_json->'product'->'images'->0))) END AS record_json`, 1)
	if err := query.Select(columns).Order("id").Limit(q.Limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		record, err := row.record()
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, record)
	}
	return page, nil
}
func (r *Repository) ListEvents(ctx context.Context, scope collection.Scope, platform, id string, q supplymarket.Query) (supplymarket.Page[supplymarket.Event], error) {
	page := supplymarket.Page[supplymarket.Event]{Items: []supplymarket.Event{}}
	if q.Validate() != nil || q.Keyword != "" || q.Kind != "" || q.Ended {
		return page, supplymarket.ErrInvalid
	}
	if _, err := r.ReadRecord(ctx, scope, platform, id); err != nil {
		return page, err
	}
	query := r.db.WithContext(ctx).Table("supply_market_events").Where("record_id=?", id)
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if q.After != "" {
		query = query.Where("id>?::uuid", q.After)
	}
	var rows []struct {
		ID        string
		EventJSON []byte
	}
	if err := query.Select("id,event_json").Order("id").Limit(q.Limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		var event supplymarket.Event
		if len(row.EventJSON) > 64<<10 || json.Unmarshal(row.EventJSON, &event) != nil || event.ID != row.ID || event.RecordID != id {
			return page, supplymarket.ErrUnavailable
		}
		page.Items = append(page.Items, event)
	}
	return page, nil
}
func (r *Repository) ListReleases(ctx context.Context, q supplymarket.Query) (supplymarket.Page[supplymarket.Release], error) {
	page := supplymarket.Page[supplymarket.Release]{Items: []supplymarket.Release{}}
	if q.Validate() != nil || q.Kind == "connection" || q.Ended {
		return page, supplymarket.ErrInvalid
	}
	query := r.db.WithContext(ctx).Table("supply_market_releases").Where("active")
	if q.Kind != "" {
		query = query.Where("channel=?", q.Kind)
	}
	if q.Keyword != "" {
		query = query.Where("release_json->'product'->>'title' ILIKE ?", keyword(q.Keyword))
	}
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if q.After != "" {
		query = query.Where("id>?::uuid", q.After)
	}
	var rows []releaseRow
	columns := strings.Replace(releaseColumns, "release_json", `jsonb_set(release_json,'{product}',jsonb_build_object('title',release_json->'product'->'title','images',jsonb_build_array(release_json->'product'->'images'->0))) AS release_json`, 1)
	if err := query.Select(columns).Order("id").Limit(q.Limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		release, err := row.release()
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, release)
	}
	return page, nil
}

var _ supplymarket.Repository = (*Repository)(nil)
