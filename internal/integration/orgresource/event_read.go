package orgresourceadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/ledger/orgresource"
)

type resourceEventCursor struct {
	Scope   string    `json:"scope"`
	Time    time.Time `json:"time"`
	EventID string    `json:"event_id"`
}

func (r *GormRepository) ListEvents(ctx context.Context, query orgresource.EventQuery) (orgresource.EventPage, error) {
	if ctx == nil || query.OrganizationID == "" || len(query.OrganizationID) > 128 || strings.TrimSpace(query.OrganizationID) != query.OrganizationID || query.Limit < 1 || query.Limit > 50 || len(query.Cursor) > 1024 {
		return orgresource.EventPage{}, orgresource.ErrInvalidInput
	}
	if query.ResourceType != "" && query.ResourceType != orgresource.ResourceStoreRenewalPeriod && query.ResourceType != orgresource.ResourceAIPoint && query.ResourceType != orgresource.ResourceDataRow {
		return orgresource.EventPage{}, orgresource.ErrInvalidInput
	}
	if (query.From != nil && query.From.IsZero()) || (query.Until != nil && query.Until.IsZero()) || (query.From != nil && query.Until != nil && !query.From.Before(*query.Until)) {
		return orgresource.EventPage{}, orgresource.ErrInvalidInput
	}
	identity, _ := json.Marshal(struct {
		Organization string
		Resource     orgresource.ResourceType
		From, Until  *time.Time
	}{query.OrganizationID, query.ResourceType, query.From, query.Until})
	hash := sha256.Sum256(identity)
	scope := hex.EncodeToString(hash[:])
	db := r.db.WithContext(ctx).Where("organization_id=?", query.OrganizationID)
	if query.ResourceType != "" {
		db = db.Where("resource_type=?", query.ResourceType)
	}
	if query.From != nil {
		db = db.Where("created_at>=?", query.From.UTC())
	}
	if query.Until != nil {
		db = db.Where("created_at<?", query.Until.UTC())
	}
	if query.Cursor != "" {
		data, err := base64.RawURLEncoding.Strict().DecodeString(query.Cursor)
		if err != nil {
			return orgresource.EventPage{}, orgresource.ErrInvalidInput
		}
		var position resourceEventCursor
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&position) != nil || decoder.Decode(new(any)) != io.EOF || position.Scope != scope || position.Time.IsZero() {
			return orgresource.EventPage{}, orgresource.ErrInvalidInput
		}
		if id, err := uuid.Parse(position.EventID); err != nil || id.String() != position.EventID {
			return orgresource.EventPage{}, orgresource.ErrInvalidInput
		}
		db = db.Where("created_at<? OR (created_at=? AND event_id<?)", position.Time, position.Time, position.EventID)
	}
	var rows []organizationResourceEventRow
	if err := db.Order("created_at DESC,event_id DESC").Limit(query.Limit + 1).Find(&rows).Error; err != nil {
		return orgresource.EventPage{}, err
	}
	page := orgresource.EventPage{OrganizationID: query.OrganizationID, Items: make([]orgresource.EventView, 0, query.Limit)}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		last := rows[len(rows)-1]
		encoded, _ := json.Marshal(resourceEventCursor{Scope: scope, Time: last.CreatedAt.UTC(), EventID: last.EventID})
		cursor := base64.RawURLEncoding.EncodeToString(encoded)
		page.NextCursor = &cursor
	}
	for _, row := range rows {
		format := func(value int64) string { return strconv.FormatInt(value, 10) }
		page.Items = append(page.Items, orgresource.EventView{EventID: row.EventID, OperationID: row.OperationID, ResourceType: orgresource.ResourceType(row.ResourceType), Quantity: format(row.Quantity), AvailableDelta: format(row.AvailableDelta), AllocatedDelta: format(row.AllocatedDelta), ReservedDelta: format(row.ReservedDelta), ConsumedDelta: format(row.ConsumedDelta), AvailableAfter: format(row.AvailableAfter), AllocatedAfter: format(row.AllocatedAfter), ReservedAfter: format(row.ReservedAfter), ConsumedAfter: format(row.ConsumedAfter), Reason: row.Reason, SourceType: row.SourceType, SourceIdentity: row.SourceIdentity, OccurredAt: row.CreatedAt.UTC()})
	}
	return page, nil
}
