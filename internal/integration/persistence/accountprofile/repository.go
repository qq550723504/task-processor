package accountprofile

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const TableName = "account_business_profiles"

type BusinessProfile struct {
	UserID           string
	UserRole         string
	ShopSituation    string
	FactorySituation string
	Platforms        []string
	Sites            []string
	ShopType         string
	Services         []string
	UpdatedAt        *time.Time
}

type AuditContext struct {
	OrganizationID string
	ActorID        string
}

type AuditEvent struct {
	ID             int64
	OrganizationID string
	ActorID        string
	UserID         string
	Operation      string
	Version        int64
	CreatedAt      time.Time
}

type AuditPosition struct {
	CreatedAt time.Time
	ID        int64
}

type profileRow struct {
	UserID           string    `gorm:"column:user_id;primaryKey"`
	UserRole         string    `gorm:"column:user_role"`
	ShopSituation    string    `gorm:"column:shop_situation"`
	FactorySituation string    `gorm:"column:factory_situation"`
	Platforms        string    `gorm:"column:platforms"`
	Sites            string    `gorm:"column:sites"`
	ShopType         string    `gorm:"column:shop_type"`
	Services         string    `gorm:"column:services"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

type auditRow struct {
	ID             uint      `gorm:"column:id;primaryKey;autoIncrement"`
	OrganizationID string    `gorm:"column:organization_id;not null;index"`
	ActorID        string    `gorm:"column:actor_id;not null"`
	UserID         string    `gorm:"column:user_id;not null"`
	Operation      string    `gorm:"column:operation;not null;size:32"`
	Version        int64     `gorm:"column:version;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index"`
}

func (auditRow) TableName() string { return "account_business_profile_audit_events" }

func (profileRow) TableName() string { return TableName }

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("account profile database is nil")
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Read(ctx context.Context, userID string) (BusinessProfile, error) {
	var row profileRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return emptyBusinessProfile(userID), nil
	}
	if err != nil {
		return BusinessProfile{}, err
	}
	profile, err := row.profile()
	if err != nil {
		return BusinessProfile{}, err
	}
	return profile, nil
}

func (r *Repository) Save(ctx context.Context, profile BusinessProfile) (BusinessProfile, error) {
	return r.SaveWithAudit(ctx, profile, AuditContext{})
}

func (r *Repository) SaveWithAudit(ctx context.Context, profile BusinessProfile, audit AuditContext) (BusinessProfile, error) {
	platforms, err := json.Marshal(profile.Platforms)
	if err != nil {
		return BusinessProfile{}, err
	}
	sites, err := json.Marshal(profile.Sites)
	if err != nil {
		return BusinessProfile{}, err
	}
	services, err := json.Marshal(profile.Services)
	if err != nil {
		return BusinessProfile{}, err
	}
	now := time.Now().UTC()
	row := profileRow{UserID: profile.UserID, UserRole: profile.UserRole, ShopSituation: profile.ShopSituation, FactorySituation: profile.FactorySituation, Platforms: string(platforms), Sites: string(sites), ShopType: profile.ShopType, Services: string(services), UpdatedAt: now}
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return BusinessProfile{}, tx.Error
	}
	defer tx.Rollback()
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"user_role", "shop_situation", "factory_situation", "platforms", "sites", "shop_type", "services", "updated_at"})}).Create(&row).Error; err != nil {
		return BusinessProfile{}, err
	}
	if audit.OrganizationID != "" && audit.ActorID != "" {
		var version int64
		if err := tx.Model(&auditRow{}).Where("organization_id = ? AND user_id = ?", audit.OrganizationID, profile.UserID).Count(&version).Error; err != nil {
			return BusinessProfile{}, err
		}
		if err := tx.Create(&auditRow{OrganizationID: audit.OrganizationID, ActorID: audit.ActorID, UserID: profile.UserID, Operation: "update", Version: version + 1, CreatedAt: now}).Error; err != nil {
			return BusinessProfile{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return BusinessProfile{}, err
	}
	result, err := row.profile()
	if err != nil {
		return BusinessProfile{}, err
	}
	return result, nil
}

func (r *Repository) ListRecentAudit(ctx context.Context, organizationID string, limit int, actor, operation string, after *AuditPosition) ([]AuditEvent, *AuditPosition, error) {
	if r == nil || r.db == nil || organizationID == "" || limit < 1 {
		return nil, nil, errors.New("invalid account profile audit request")
	}
	query := r.db.WithContext(ctx).Where("organization_id = ?", organizationID)
	if actor != "" {
		query = query.Where("actor_id = ?", actor)
	}
	if operation != "" {
		query = query.Where("operation = ?", operation)
	}
	if after != nil {
		query = query.Where("(created_at < ? OR (created_at = ? AND id < ?))", after.CreatedAt, after.CreatedAt, after.ID)
	}
	var rows []auditRow
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	events := make([]AuditEvent, 0, limit)
	for index, row := range rows {
		if index == limit {
			position := events[len(events)-1].Position()
			return events, &position, nil
		}
		events = append(events, AuditEvent{ID: int64(row.ID), OrganizationID: row.OrganizationID, ActorID: row.ActorID, UserID: row.UserID, Operation: row.Operation, Version: row.Version, CreatedAt: row.CreatedAt.UTC().Truncate(time.Microsecond)})
	}
	return events, nil, nil
}

func (e AuditEvent) Position() AuditPosition {
	return AuditPosition{CreatedAt: e.CreatedAt.UTC().Truncate(time.Microsecond), ID: e.ID}
}

func (row profileRow) profile() (BusinessProfile, error) {
	var platforms, sites, services []string
	if err := json.Unmarshal([]byte(row.Platforms), &platforms); err != nil {
		return BusinessProfile{}, err
	}
	if err := json.Unmarshal([]byte(row.Sites), &sites); err != nil {
		return BusinessProfile{}, err
	}
	if err := json.Unmarshal([]byte(row.Services), &services); err != nil {
		return BusinessProfile{}, err
	}
	updated := row.UpdatedAt.UTC()
	if platforms == nil {
		platforms = []string{}
	}
	if sites == nil {
		sites = []string{}
	}
	if services == nil {
		services = []string{}
	}
	return BusinessProfile{UserID: row.UserID, UserRole: row.UserRole, ShopSituation: row.ShopSituation, FactorySituation: row.FactorySituation, Platforms: platforms, Sites: sites, ShopType: row.ShopType, Services: services, UpdatedAt: &updated}, nil
}

func emptyBusinessProfile(userID string) BusinessProfile {
	return BusinessProfile{UserID: userID, Platforms: []string{}, Sites: []string{}, Services: []string{}}
}
