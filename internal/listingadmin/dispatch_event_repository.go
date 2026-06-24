package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const dispatchEventMaxLimit = 200

type DispatchEventQuery struct {
	TenantID   int64
	Platform   string
	StoreID    *int64
	Action     string
	ReasonCode string
	From       time.Time
	To         time.Time
	Page       int
	PageSize   int
	Limit      int
	Offset     int
}

type DispatchEventFilter = DispatchEventQuery

type DispatchEventWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type DispatchEventReasonCount struct {
	ReasonCode string `json:"reasonCode"`
	Action     string `json:"action"`
	Count      int64  `json:"count"`
}

type DispatchEventSummaryRow = DispatchEventReasonCount

type DispatchEventStoreBlocker struct {
	TenantID          int64  `json:"tenantId"`
	StoreID           int64  `json:"storeId"`
	ReasonCode        string `json:"reasonCode"`
	Count             int64  `json:"count"`
	DailyLimit        int    `json:"dailyLimit"`
	MaxQueued         int64  `json:"maxQueued"`
	MaxProcessing     int    `json:"maxProcessing"`
	MaxCompletedToday int    `json:"maxCompletedToday"`
	OwnerNode         string `json:"ownerNode,omitempty"`
}

type DispatchEventSummary struct {
	Window        DispatchEventWindow         `json:"window"`
	Total         int64                       `json:"total"`
	Dispatched    int64                       `json:"dispatched"`
	Skipped       int64                       `json:"skipped"`
	Failed        int64                       `json:"failed"`
	ReasonCounts  []DispatchEventReasonCount  `json:"reasonCounts"`
	StoreBlockers []DispatchEventStoreBlocker `json:"storeBlockers"`
}

type DispatchEventItem struct {
	ID             int64     `json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	TaskID         int64     `json:"taskId"`
	TenantID       int64     `json:"tenantId"`
	StoreID        int64     `json:"storeId"`
	Platform       string    `json:"platform,omitempty"`
	Action         string    `json:"action"`
	ReasonCode     string    `json:"reasonCode,omitempty"`
	Stage          string    `json:"stage,omitempty"`
	Capacity       int       `json:"capacity"`
	Queued         int64     `json:"queued"`
	Processing     int       `json:"processing"`
	CompletedToday int       `json:"completedToday"`
	DailyLimit     int       `json:"dailyLimit"`
	OwnerNode      string    `json:"ownerNode,omitempty"`
}

type DispatchEventListRow = DispatchEventItem

type DispatchEventPage struct {
	Items    []DispatchEventItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Limit    int                 `json:"limit"`
	Offset   int                 `json:"offset"`
}

type DispatchEventRepository interface {
	GetDispatchEventSummary(ctx context.Context, query DispatchEventQuery) (*DispatchEventSummary, error)
	ListDispatchEvents(ctx context.Context, query DispatchEventQuery) (*DispatchEventPage, error)
}

type GormDispatchEventRepository struct {
	db *gorm.DB
}

func NewGormDispatchEventRepository(db *gorm.DB) *GormDispatchEventRepository {
	return &GormDispatchEventRepository{db: db}
}

func (r *GormDispatchEventRepository) GetDispatchEventSummary(ctx context.Context, query DispatchEventQuery) (*DispatchEventSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("dispatch event repository database is not configured")
	}
	query = normalizeDispatchEventQuery(query)
	if query.TenantID <= 0 {
		return nil, errors.New("dispatch event tenant id is required")
	}
	base := applyDispatchEventQuery(r.db.WithContext(ctx).Table("listing_dispatch_event"), query)

	var actionRows []struct {
		Action string
		Count  int64
	}
	if err := base.Select("action, count(*) as count").Group("action").Scan(&actionRows).Error; err != nil {
		return nil, err
	}

	var total, dispatched, skipped, failed int64
	for _, row := range actionRows {
		total += row.Count
		switch strings.TrimSpace(row.Action) {
		case "dispatched":
			dispatched += row.Count
		case "skipped":
			skipped += row.Count
		case "failed":
			failed += row.Count
		}
	}

	var reasonRows []DispatchEventReasonCount
	if err := base.
		Where("reason_code <> ''").
		Select("reason_code, action, count(*) as count").
		Group("reason_code, action").
		Order("count desc, reason_code asc, action asc").
		Scan(&reasonRows).Error; err != nil {
		return nil, err
	}

	var blockerRows []DispatchEventStoreBlocker
	if err := base.
		Where("action = ?", "skipped").
		Select(`tenant_id, store_id, reason_code, count(*) as count,
			max(daily_limit) as daily_limit,
			max(queued) as max_queued,
			max(processing) as max_processing,
			max(completed_today) as max_completed_today,
			max(owner_node) as owner_node`).
		Group("tenant_id, store_id, reason_code").
		Order("count desc, tenant_id asc, store_id asc, reason_code asc").
		Limit(20).
		Scan(&blockerRows).Error; err != nil {
		return nil, err
	}

	return &DispatchEventSummary{
		Window:        DispatchEventWindow{From: query.From, To: query.To},
		Total:         total,
		Dispatched:    dispatched,
		Skipped:       skipped,
		Failed:        failed,
		ReasonCounts:  reasonRows,
		StoreBlockers: blockerRows,
	}, nil
}

func (r *GormDispatchEventRepository) ListDispatchEvents(ctx context.Context, query DispatchEventQuery) (*DispatchEventPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("dispatch event repository database is not configured")
	}
	query = normalizeDispatchEventQuery(query)
	if query.TenantID <= 0 {
		return nil, errors.New("dispatch event tenant id is required")
	}
	base := applyDispatchEventQuery(r.db.WithContext(ctx).Table("listing_dispatch_event"), query)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []listingDispatchEvent
	if err := base.
		Order("created_at desc, id desc").
		Offset(query.Offset).
		Limit(query.Limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]DispatchEventItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dispatchEventItemFromRow(row))
	}
	return &DispatchEventPage{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize, Limit: query.Limit, Offset: query.Offset}, nil
}

func normalizeDispatchEventQuery(query DispatchEventQuery) DispatchEventQuery {
	query.Platform = strings.TrimSpace(query.Platform)
	query.Action = strings.TrimSpace(query.Action)
	query.ReasonCode = strings.TrimSpace(query.ReasonCode)
	if query.Limit <= 0 {
		query.Limit = query.PageSize
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > dispatchEventMaxLimit {
		query.Limit = dispatchEventMaxLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	if query.Offset == 0 && query.Page > 1 {
		query.Offset = (query.Page - 1) * query.Limit
	}
	if query.Page <= 0 {
		query.Page = query.Offset/query.Limit + 1
	}
	query.PageSize = query.Limit
	return query
}

func applyDispatchEventQuery(db *gorm.DB, query DispatchEventQuery) *gorm.DB {
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.ReasonCode != "" {
		db = db.Where("reason_code = ?", query.ReasonCode)
	}
	if !query.From.IsZero() {
		db = db.Where("created_at >= ?", query.From)
	}
	if !query.To.IsZero() {
		db = db.Where("created_at <= ?", query.To)
	}
	return db
}

func dispatchEventItemFromRow(row listingDispatchEvent) DispatchEventItem {
	return DispatchEventItem{
		ID:             row.ID,
		CreatedAt:      timeFromPointer(row.CreatedAt),
		TaskID:         row.TaskID,
		TenantID:       row.TenantID,
		StoreID:        row.StoreID,
		Platform:       row.Platform,
		Action:         row.Action,
		ReasonCode:     row.ReasonCode,
		Stage:          row.Stage,
		Capacity:       row.Capacity,
		Queued:         row.Queued,
		Processing:     row.Processing,
		CompletedToday: row.CompletedToday,
		DailyLimit:     row.DailyLimit,
		OwnerNode:      row.OwnerNode,
	}
}

func timeFromPointer(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
