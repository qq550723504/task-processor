package sheinobservations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/authidentity"
	o "task-processor/internal/marketplace/shein/observations"
	"time"
)

type Repository struct{ db *gorm.DB }
type syncRow struct {
	OrganizationID, StoreID, ID, CommandID, ActorID, MemberID, CommandKey, Kind, Status, ErrorCode string
	Generation, Revision                                                                           int64
	Binding, Progress, QueryRange                                                                  []byte
	CreatedAt                                                                                      time.Time
	ObservedAt                                                                                     *time.Time
}

func (x syncRow) value() (o.Sync, error) {
	s := o.Sync{ID: x.ID, CommandID: x.CommandID, StoreID: x.StoreID, Owner: o.Scope{OrganizationID: x.OrganizationID, ActorID: x.ActorID, MemberID: x.MemberID}, Key: strings.TrimSpace(x.CommandKey), Kind: o.Kind(x.Kind), Status: x.Status, ErrorCode: x.ErrorCode, Generation: x.Generation, Revision: x.Revision, CreatedAt: x.CreatedAt, ObservedAt: x.ObservedAt}
	if json.Unmarshal(x.Binding, &s.Binding) != nil || json.Unmarshal(x.Progress, &s.Progress) != nil {
		return s, o.ErrUnavailable
	}
	if len(x.QueryRange) > 0 && string(x.QueryRange) != "null" && json.Unmarshal(x.QueryRange, &s.Range) != nil {
		return s, o.ErrUnavailable
	}
	return s, nil
}
func readSync(db *gorm.DB, org, id string, lock bool) (o.Sync, error) {
	sql := `SELECT * FROM shein_observations.syncs WHERE organization_id=? AND id=?`
	if lock {
		sql += " FOR UPDATE"
	}
	var x syncRow
	r := db.Raw(sql, org, id).Scan(&x)
	if r.Error != nil {
		return o.Sync{}, o.ErrUnavailable
	}
	if r.RowsAffected == 0 {
		return o.Sync{}, o.ErrNotFound
	}
	return x.value()
}
func (r *Repository) ReadSync(ctx context.Context, org, id string) (o.Sync, error) {
	if !authidentity.IsBoundedIdentifier(org) || !o.ValidID(id) {
		return o.Sync{}, o.ErrInvalid
	}
	return readSync(r.db.WithContext(ctx), org, id, false)
}

type commandRow struct {
	OrganizationID, ID, ActorID, MemberID, CommandKey, PayloadHash string
	Input                                                          []byte
	CreatedAt                                                      time.Time
}

func command(db *gorm.DB, scope o.Scope, key string) (o.Command, error) {
	var x commandRow
	result := db.Raw(`SELECT * FROM shein_observations.commands WHERE organization_id=? AND actor_id=? AND member_id=? AND command_key=?`, scope.OrganizationID, scope.ActorID, scope.MemberID, key).Scan(&x)
	if result.Error != nil {
		return o.Command{}, o.ErrUnavailable
	}
	if result.RowsAffected == 0 {
		return o.Command{}, o.ErrNotFound
	}
	c := o.Command{ID: x.ID, Owner: scope, Key: key, Hash: strings.TrimSpace(x.PayloadHash), CreatedAt: x.CreatedAt, Syncs: []o.Sync{}}
	if json.Unmarshal(x.Input, &c.Input) != nil {
		return c, o.ErrUnavailable
	}
	var rows []syncRow
	if db.Raw(`SELECT * FROM shein_observations.syncs WHERE organization_id=? AND command_id=? ORDER BY store_id`, scope.OrganizationID, c.ID).Scan(&rows).Error != nil {
		return c, o.ErrUnavailable
	}
	for _, row := range rows {
		s, e := row.value()
		if e != nil {
			return c, e
		}
		c.Syncs = append(c.Syncs, s)
	}
	return c, nil
}
func (r *Repository) CommandByKey(ctx context.Context, scope o.Scope, key string) (o.Command, error) {
	if !scope.Valid() || !o.ValidID(key) {
		return o.Command{}, o.ErrInvalid
	}
	return command(r.db.WithContext(ctx), scope, key)
}
func (r *Repository) Begin(ctx context.Context, scope o.Scope, key, hash string, in o.BeginInput, children []o.Sync) (o.Command, error) {
	if !scope.Valid() || !o.ValidID(key) || !o.Text(hash, 64) || hash == "" || !in.Kind.Valid() || len(children) == 0 || len(children) > 500 || len(children) != len(in.Stores) {
		return o.Command{}, o.ErrInvalid
	}
	var out o.Command
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serializes only this original member's parent operation, including a lost
		// commit response. A replacement membership never inherits its receipt.
		lockJSON, _ := json.Marshal([]string{scope.OrganizationID, scope.ActorID, key})
		lockHash := sha256.Sum256(lockJSON)
		if tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, hex.EncodeToString(lockHash[:])).Error != nil {
			return o.ErrUnavailable
		}
		prior, e := command(tx, scope, key)
		if e == nil {
			if prior.Hash != hash {
				return o.ErrConflict
			}
			out = prior
			return nil
		}
		if !errors.Is(e, o.ErrNotFound) {
			return e
		}
		var used int64
		if tx.Raw(`SELECT count(*) FROM shein_observations.commands WHERE organization_id=? AND actor_id=? AND command_key=?`, scope.OrganizationID, scope.ActorID, key).Scan(&used).Error != nil {
			return o.ErrUnavailable
		}
		if used != 0 {
			return o.ErrConflict
		}
		raw, e := json.Marshal(in)
		if e != nil || len(raw) > 131072 {
			return o.ErrInvalid
		}
		id := uuid.NewString()
		at := children[0].CreatedAt
		if tx.Exec(`INSERT INTO shein_observations.commands(organization_id,id,actor_id,member_id,command_key,payload_hash,input,created_at) VALUES(?,?,?,?,?,?,?::jsonb,?)`, scope.OrganizationID, id, scope.ActorID, scope.MemberID, key, hash, string(raw), at).Error != nil {
			return o.ErrUnavailable
		}
		seen := map[string]bool{}
		for i, s := range children {
			if !o.ValidID(s.ID) || !o.ValidID(s.StoreID) || s.StoreID != in.Stores[i] || seen[s.StoreID] || s.Kind != in.Kind || s.Owner != scope || (s.Status != "pending" && s.Status != "suspended") || s.Progress.Page != 1 || !o.Text(s.Key, 64) || s.Key == "" {
				return o.ErrInvalid
			}
			seen[s.StoreID] = true
			var generation int64
			if tx.Raw(`INSERT INTO shein_observations.heads(organization_id,store_id,kind,sequence) VALUES(?,?,?,1) ON CONFLICT(organization_id,store_id,kind) DO UPDATE SET sequence=shein_observations.heads.sequence+1 RETURNING sequence`, scope.OrganizationID, s.StoreID, s.Kind).Scan(&generation).Error != nil {
				return o.ErrUnavailable
			}
			binding, _ := json.Marshal(s.Binding)
			progress, _ := json.Marshal(s.Progress)
			queryRange, _ := json.Marshal(s.Range)
			if tx.Exec(`INSERT INTO shein_observations.syncs(organization_id,store_id,id,command_id,actor_id,member_id,command_key,kind,generation,revision,status,binding,progress,query_range,created_at,error_code) VALUES(?,?,?,?,?,?,?,?,?,1,?,?::jsonb,?::jsonb,?::jsonb,?,?)`, scope.OrganizationID, s.StoreID, s.ID, id, scope.ActorID, scope.MemberID, s.Key, s.Kind, generation, s.Status, string(binding), string(progress), string(queryRange), s.CreatedAt, s.ErrorCode).Error != nil {
				return o.ErrUnavailable
			}
		}
		out, e = command(tx, scope, key)
		return e
	})
	return out, err
}
func (r *Repository) Heads(ctx context.Context, org string, kind o.Kind, stores []string) ([]o.Sync, []o.Sync, error) {
	selected, latest := []o.Sync{}, []o.Sync{}
	if !authidentity.IsBoundedIdentifier(org) || !kind.Valid() {
		return nil, nil, o.ErrInvalid
	}
	if len(stores) == 0 {
		return selected, latest, nil
	}
	for _, id := range stores {
		if !o.ValidID(id) {
			return nil, nil, o.ErrInvalid
		}
	}
	db := r.db.WithContext(ctx)
	var rows []syncRow
	if db.Raw(`SELECT DISTINCT ON(store_id) * FROM shein_observations.syncs WHERE organization_id=? AND kind=? AND store_id IN ? ORDER BY store_id,generation DESC`, org, kind, stores).Scan(&rows).Error != nil {
		return nil, nil, o.ErrUnavailable
	}
	byStore := map[string]o.Sync{}
	for _, row := range rows {
		s, e := row.value()
		if e != nil {
			return nil, nil, e
		}
		latest = append(latest, s)
		byStore[s.StoreID] = s
	}
	rows = nil
	if db.Raw(`SELECT s.* FROM shein_observations.heads h JOIN shein_observations.syncs s ON s.organization_id=h.organization_id AND s.store_id=h.store_id AND s.id=h.current_id AND s.kind=h.kind WHERE h.organization_id=? AND h.kind=? AND h.store_id IN ?`, org, kind, stores).Scan(&rows).Error != nil {
		return nil, nil, o.ErrUnavailable
	}
	for _, row := range rows {
		s, e := row.value()
		if e != nil {
			return nil, nil, e
		}
		byStore[s.StoreID] = s
	}
	for _, store := range stores {
		if s, ok := byStore[store]; ok {
			selected = append(selected, s)
		}
	}
	return selected, latest, nil
}
func sameSync(a, b o.Sync) bool {
	return a.ID == b.ID && a.StoreID == b.StoreID && a.Owner == b.Owner && a.Kind == b.Kind && a.Binding == b.Binding && a.Generation == b.Generation && a.Revision == b.Revision
}
func (r *Repository) CommitPage(ctx context.Context, expected o.Sync, next o.Checkpoint, records []o.Record, status string, at time.Time) (o.Sync, error) {
	if status != "running" && status != "completed" && status != "partial" || !expected.Owner.Valid() || !o.ValidID(expected.ID) || at.IsZero() || len(records) > 30 {
		return o.Sync{}, o.ErrInvalid
	}
	var out o.Sync
	e := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, e := readSync(tx, expected.Owner.OrganizationID, expected.ID, true)
		if e != nil {
			return e
		}
		if !sameSync(current, expected) || current.Terminal() {
			return o.ErrConflict
		}
		// Coverage defects cannot be cleared by a later successful page.
		for _, note := range current.Progress.Notes {
			next.Note(note)
		}
		next.Incomplete = next.Incomplete || current.Progress.Incomplete
		seen := map[string]bool{}
		for _, v := range records {
			if v.StoreID != current.StoreID || v.SyncID != current.ID || !o.Identity(v.ID) || seen[v.ID] || at.Before(current.CreatedAt) {
				return o.ErrInvalid
			}
			seen[v.ID] = true
			if current.Kind == o.Products && (v.Product == nil || v.Order != nil || v.Product.ID != v.ID) || current.Kind == o.Orders && (v.Order == nil || v.Product != nil || v.Order.ID != v.ID || v.Order.Site != "shein-us") {
				return o.ErrInvalid
			}
			var prior struct {
				WindowKey string
				PageIndex int
			}
			result := tx.Raw(`SELECT window_key,page_index FROM shein_observations.records WHERE organization_id=? AND store_id=? AND sync_id=? AND platform_id=?`, current.Owner.OrganizationID, current.StoreID, current.ID, v.ID).Scan(&prior)
			if result.Error != nil {
				return o.ErrUnavailable
			}
			if result.RowsAffected > 0 && prior.WindowKey == v.WindowKey && prior.PageIndex != current.Progress.Page {
				next.Note("duplicate_platform_identity")
			}
			payload, _ := json.Marshal(struct {
				Product *o.Product `json:"product,omitempty"`
				Order   *o.Order   `json:"order,omitempty"`
			}{v.Product, v.Order})
			if len(payload) > 262144 {
				return o.ErrInvalid
			}
			if tx.Exec(`INSERT INTO shein_observations.records(organization_id,store_id,sync_id,kind,platform_id,payload,observed_at,window_key,page_index) VALUES(?,?,?,?,?,?::jsonb,?,?,?) ON CONFLICT(organization_id,store_id,sync_id,platform_id) DO UPDATE SET payload=EXCLUDED.payload,observed_at=EXCLUDED.observed_at,window_key=EXCLUDED.window_key,page_index=EXCLUDED.page_index`, current.Owner.OrganizationID, current.StoreID, current.ID, current.Kind, v.ID, string(payload), at, v.WindowKey, current.Progress.Page).Error != nil {
				return o.ErrUnavailable
			}
		}
		if status == "completed" && next.Incomplete {
			status = "partial"
		}
		raw, _ := json.Marshal(next)
		if len(raw) > 65536 {
			return o.ErrInvalid
		}
		if tx.Exec(`UPDATE shein_observations.syncs SET progress=?::jsonb,status=?,revision=revision+1,observed_at=? WHERE organization_id=? AND id=?`, string(raw), status, at, current.Owner.OrganizationID, current.ID).Error != nil {
			return o.ErrUnavailable
		}
		if status == "completed" {
			// Allocation and promotion share this scoped head lock. An older completion
			// can retain its own results but cannot undo the newer published generation.
			if tx.Exec(`UPDATE shein_observations.heads h SET current_id=? WHERE h.organization_id=? AND h.store_id=? AND h.kind=? AND (h.current_id IS NULL OR EXISTS(SELECT 1 FROM shein_observations.syncs s WHERE s.organization_id=h.organization_id AND s.store_id=h.store_id AND s.kind=h.kind AND s.id=h.current_id AND s.generation<?))`, current.ID, current.Owner.OrganizationID, current.StoreID, current.Kind, current.Generation).Error != nil {
				return o.ErrUnavailable
			}
		}
		out, e = readSync(tx, current.Owner.OrganizationID, current.ID, false)
		return e
	})
	return out, e
}
func (r *Repository) Stop(ctx context.Context, expected o.Sync, status, code string) (o.Sync, error) {
	if (status != "failed" && status != "suspended") || !o.Text(code, 64) || !expected.Owner.Valid() || !o.ValidID(expected.ID) {
		return o.Sync{}, o.ErrInvalid
	}
	var out o.Sync
	e := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, e := readSync(tx, expected.Owner.OrganizationID, expected.ID, true)
		if e != nil {
			return e
		}
		if current.Terminal() {
			out = current
			return nil
		}
		if !sameSync(current, expected) {
			return o.ErrConflict
		}
		if tx.Exec(`UPDATE shein_observations.syncs SET status=?,error_code=?,revision=revision+1 WHERE organization_id=? AND id=?`, status, code, current.Owner.OrganizationID, current.ID).Error != nil {
			return o.ErrUnavailable
		}
		out, e = readSync(tx, current.Owner.OrganizationID, current.ID, false)
		return e
	})
	return out, e
}

type recordRow struct {
	StoreID, SyncID, PlatformID, RowID string
	Payload                            []byte
	ObservedAt                         time.Time
}

func (x recordRow) value() (o.Record, error) {
	v := o.Record{ID: x.PlatformID, StoreID: x.StoreID, SyncID: x.SyncID, ObservedAt: x.ObservedAt}
	if json.Unmarshal(x.Payload, &v) != nil {
		return v, o.ErrUnavailable
	}
	return v, nil
}
func (r *Repository) Record(ctx context.Context, org, store string, kind o.Kind, sync, id string) (o.Record, error) {
	if !authidentity.IsBoundedIdentifier(org) || !o.ValidID(store) || !kind.Valid() || !o.ValidID(sync) || !o.Identity(id) {
		return o.Record{}, o.ErrInvalid
	}
	var row recordRow
	q := r.db.WithContext(ctx).Raw(`SELECT store_id,sync_id,platform_id,payload,observed_at FROM shein_observations.records WHERE organization_id=? AND store_id=? AND kind=? AND sync_id=? AND platform_id=?`, org, store, kind, sync, id).Scan(&row)
	if q.Error != nil {
		return o.Record{}, o.ErrUnavailable
	}
	if q.RowsAffected == 0 {
		return o.Record{}, o.ErrNotFound
	}
	return row.value()
}

type pageCursor struct {
	Scope string `json:"scope"`
	Store string `json:"store"`
	ID    string `json:"id"`
	Row   string `json:"row"`
}

func queryHash(org string, q o.Query) string {
	raw, _ := json.Marshal(struct {
		Org             string
		Kind            o.Kind
		Keyword, Status string
		Sources         map[string]string
	}{org, q.Kind, q.Keyword, q.Status, q.Sources})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (r *Repository) List(ctx context.Context, org string, q o.Query) (out o.Result, err error) {
	// Pending generations can receive a page while read. Summary and rows still
	// come from one SQL snapshot; completed generations remain immutable.
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var e error; out, e = (&Repository{db: tx}).list(ctx, org, q); return e }, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}
func (r *Repository) list(ctx context.Context, org string, q o.Query) (o.Result, error) {
	out := o.Result{Items: []o.Record{}}
	if !authidentity.IsBoundedIdentifier(org) || !q.Kind.Valid() || q.Limit < 1 || q.Limit > 50 || !o.Text(q.Keyword, 200) || len(q.After) > 4096 || len(q.Sources) > 500 || q.RecordByteLimit < 0 || q.RecordByteLimit > 1<<20 {
		return out, o.ErrInvalid
	}
	fingerprint := queryHash(org, q)
	var cursor pageCursor
	if q.After != "" {
		raw, e := base64.RawURLEncoding.DecodeString(q.After)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Scope != fingerprint || !o.ValidID(cursor.Store) || !o.Identity(cursor.ID) || !o.Text(cursor.Row, 128) {
			return out, o.ErrConflict
		}
	}
	if len(q.Sources) == 0 {
		return out, nil
	}
	conditions, args := []string{}, []any{org, q.Kind}
	for store, sync := range q.Sources {
		if !o.ValidID(store) || !o.ValidID(sync) {
			return out, o.ErrInvalid
		}
		conditions = append(conditions, "(r.store_id=? AND r.sync_id=?)")
		args = append(args, store, sync)
	}
	base := `SELECT r.store_id,r.sync_id,r.platform_id,r.observed_at,`
	if q.Kind == o.Products {
		base += `k->>'id' AS row_id,jsonb_build_object('product',jsonb_build_object('id',r.platform_id,'skcs',jsonb_build_array(k))) AS payload,k AS item FROM shein_observations.records r CROSS JOIN LATERAL jsonb_array_elements(r.payload->'product'->'skcs') k`
	} else {
		base += `r.platform_id AS row_id,r.payload,r.payload->'order' AS item FROM shein_observations.records r`
	}
	base += ` WHERE r.organization_id=? AND r.kind=? AND (` + strings.Join(conditions, " OR ") + `)`
	filter := " WHERE true"
	if q.Keyword != "" {
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q.Keyword) + "%"
		if q.Kind == o.Products {
			filter += ` AND (platform_id ILIKE ? OR row_id ILIKE ? OR item->>'title' ILIKE ? OR item->>'sellerCode' ILIKE ? OR EXISTS(SELECT 1 FROM jsonb_array_elements(item->'skus') sku WHERE sku->>'id' ILIKE ? OR sku->>'sellerSku' ILIKE ?))`
			for i := 0; i < 6; i++ {
				args = append(args, pattern)
			}
		} else {
			filter += ` AND (platform_id ILIKE ? OR EXISTS(SELECT 1 FROM jsonb_array_elements(item->'items') goods WHERE goods->>'title' ILIKE ? OR goods->>'sku' ILIKE ? OR goods->>'sellerSku' ILIKE ?))`
			for i := 0; i < 4; i++ {
				args = append(args, pattern)
			}
		}
	}
	exceptional := `(jsonb_array_length(item->'reasons')>0 OR item->>'tag'='1' OR item->>'status' IN('8','9') OR EXISTS(SELECT 1 FROM jsonb_array_elements(item->'packages') p WHERE p->>'label' IN('1','2')))`
	if q.Status != "" {
		if q.Kind == o.Products {
			switch q.Status {
			case "active":
				filter += ` AND item->>'siteStatus'='1'`
			case "off":
				filter += ` AND item->>'siteStatus'='0'`
			case "unknown":
				filter += ` AND (item->>'siteStatus' IS NULL OR item->>'siteStatus' NOT IN('0','1'))`
			default:
				return out, o.ErrInvalid
			}
		} else {
			switch q.Status {
			case "transit":
				filter += ` AND item->>'status' IN('4','7')`
			case "exceptional":
				filter += " AND " + exceptional
			case "unknown":
				filter += ` AND (item->>'status' IS NULL OR item->>'status' NOT IN('1','2','3','4','5','6','7','8','9'))`
			default:
				if !strings.Contains("|1|2|3|4|5|6|7|8|9|", "|"+q.Status+"|") {
					return out, o.ErrInvalid
				}
				filter += ` AND item->>'status'=?`
				args = append(args, q.Status)
			}
		}
	}
	cte := `WITH rows AS (` + base + `), filtered AS (SELECT * FROM rows` + filter + `) `
	summary := `SELECT count(*) AS total`
	summaryArgs := append([]any{}, args...)
	if q.Kind == o.Products {
		summary += `,count(*) FILTER(WHERE item->>'siteStatus'='1') AS active,count(*) FILTER(WHERE item->>'siteStatus'='0') AS off_shelf,count(*) FILTER(WHERE item->>'siteStatus' IS NULL OR item->>'siteStatus' NOT IN('0','1')) AS unknown`
	} else {
		summary += `,count(*) FILTER(WHERE NULLIF(item->>'createdAt','')::timestamptz>=? AND NULLIF(item->>'createdAt','')::timestamptz<?) AS today,count(*) FILTER(WHERE NULLIF(item->>'createdAt','') IS NULL) AS today_unknown,count(*) FILTER(WHERE item->>'status' IN('1','2','3')) AS pending,count(*) FILTER(WHERE item->>'status' IN('4','7')) AS transit,count(*) FILTER(WHERE ` + exceptional + `) AS exceptional,count(*) FILTER(WHERE item->>'status' IS NULL OR item->>'status' NOT IN('1','2','3','4','5','6','7','8','9')) AS unknown`
		summaryArgs = append(summaryArgs, q.TodayStart, q.TodayStart.Add(24*time.Hour))
	}
	if r.db.WithContext(ctx).Raw(cte+summary+` FROM filtered`, summaryArgs...).Scan(&out.Summary).Error != nil {
		return out, o.ErrUnavailable
	}
	page := cte + `SELECT store_id,sync_id,platform_id,row_id,payload,observed_at FROM filtered`
	if q.After != "" {
		page += ` WHERE (store_id::text,platform_id,row_id)>(?,?,?)`
		args = append(args, cursor.Store, cursor.ID, cursor.Row)
	}
	page += ` ORDER BY store_id,platform_id,row_id LIMIT ?`
	args = append(args, q.Limit+1)
	var rows []recordRow
	if r.db.WithContext(ctx).Raw(page, args...).Scan(&rows).Error != nil {
		return out, o.ErrUnavailable
	}
	// Bound encoded observations as well as row count. A legitimate many-SKU
	// product must remain pageable instead of making every HTTP response too big.
	encodedBytes, more := 2, false
	byteLimit := 1 << 20
	if q.RecordByteLimit > 0 {
		byteLimit = q.RecordByteLimit
	}
	for _, row := range rows {
		if len(out.Items) == q.Limit {
			more = true
			break
		}
		v, e := row.value()
		if e != nil {
			return out, e
		}
		payload, e := json.Marshal(v)
		if e != nil {
			return out, o.ErrUnavailable
		}
		size := encodedBytes + len(payload)
		if len(out.Items) > 0 {
			size++
		}
		if size > byteLimit {
			if len(out.Items) == 0 {
				return out, o.ErrUnavailable
			}
			more = true
			break
		}
		encodedBytes = size
		out.Items = append(out.Items, v)
	}
	if more {
		last := rows[len(out.Items)-1]
		raw, _ := json.Marshal(pageCursor{fingerprint, last.StoreID, last.PlatformID, last.RowID})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}

var _ o.Repository = (*Repository)(nil)
