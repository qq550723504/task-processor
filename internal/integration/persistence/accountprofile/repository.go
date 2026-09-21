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
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"user_role", "shop_situation", "factory_situation", "platforms", "sites", "shop_type", "services", "updated_at"})}).Create(&row).Error; err != nil {
		return BusinessProfile{}, err
	}
	result, err := row.profile()
	if err != nil {
		return BusinessProfile{}, err
	}
	return result, nil
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
