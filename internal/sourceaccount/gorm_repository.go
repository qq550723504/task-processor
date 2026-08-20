package sourceaccount

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

type sourceAccountRow struct {
	ID             int64  `gorm:"primaryKey"`
	TenantID       int64  `gorm:"not null;index:idx_source_account_tenant_platform,priority:1;index:idx_source_account_tenant_id,priority:1"`
	Platform       string `gorm:"size:32;not null;index:idx_source_account_tenant_platform,priority:2"`
	Label          string `gorm:"size:128"`
	ProfileRef     string `gorm:"size:256;not null"`
	ProxyRef       string `gorm:"size:256"`
	LoginURL       string `gorm:"type:text"`
	Status         int16  `gorm:"not null;default:0;index:idx_source_account_tenant_platform,priority:3"`
	Deleted        int16  `gorm:"not null;default:0;index:idx_source_account_tenant_platform,priority:4;index:idx_source_account_tenant_id,priority:3"`
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (sourceAccountRow) TableName() string { return "source_account" }

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func AutoMigrateRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("source account database is nil")
	}
	return db.AutoMigrate(&sourceAccountRow{})
}

func (r *GormRepository) Get(ctx context.Context, tenantID, accountID int64) (*SourceAccount, error) {
	if r == nil || r.db == nil || tenantID <= 0 || accountID <= 0 {
		return nil, NewUnavailableError("")
	}
	var row sourceAccountRow
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND LOWER(platform) = ?", tenantID, accountID, PlatformAlibaba1688).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewUnavailableError("")
	}
	if err != nil {
		return nil, NewUnavailableError("")
	}
	if row.Deleted != 0 {
		return nil, NewUnavailableError("")
	}
	if row.Status != StatusEnabled {
		return nil, NewDisabledError()
	}
	if strings.TrimSpace(row.ProfileRef) == "" {
		return nil, NewUnavailableError("")
	}
	account := row.toAccount()
	return &account, nil
}

func (r *GormRepository) ValidateSourceAccountAccess(ctx context.Context, tenantID, accountID int64) (Access, error) {
	account, err := r.Get(ctx, tenantID, accountID)
	if err != nil {
		return Access{}, err
	}
	return Access{ID: account.ID, TenantID: account.TenantID, Platform: account.Platform, Enabled: true}, nil
}

func (r sourceAccountRow) toAccount() SourceAccount {
	return SourceAccount{
		ID:             r.ID,
		TenantID:       r.TenantID,
		Platform:       strings.ToLower(strings.TrimSpace(r.Platform)),
		Label:          strings.TrimSpace(r.Label),
		ProfileRef:     strings.TrimSpace(r.ProfileRef),
		ProxyRef:       strings.TrimSpace(r.ProxyRef),
		LoginURL:       strings.TrimSpace(r.LoginURL),
		Status:         r.Status,
		Deleted:        r.Deleted,
		LastVerifiedAt: r.LastVerifiedAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}
