package memberinvite

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Outcome string

const (
	OutcomeSucceeded  Outcome = "succeeded"
	OutcomeFailed     Outcome = "failed"
	OutcomeIncomplete Outcome = "incomplete"
)

// AuditRecord is the non-sensitive, durable record of one member invitation attempt.
type AuditRecord struct {
	ActorUserID     string
	TenantID        string
	Email           string
	Role            string
	UserID          string
	AuthorizationID string
	Outcome         Outcome
	ErrorCode       string
}

// AuditRepository appends a durable audit event for each invitation outcome.
type AuditRepository interface {
	Record(context.Context, AuditRecord) error
}

type memberInvitationAuditRow struct {
	ID              uint64    `gorm:"primaryKey"`
	ActorUserID     string    `gorm:"type:varchar(128);not null;index"`
	TenantID        string    `gorm:"type:varchar(128);not null;index"`
	Email           string    `gorm:"type:varchar(320);not null;index"`
	Role            string    `gorm:"type:varchar(64);not null"`
	UserID          string    `gorm:"type:varchar(128)"`
	AuthorizationID string    `gorm:"type:varchar(128)"`
	Outcome         Outcome   `gorm:"type:varchar(32);not null;index"`
	ErrorCode       string    `gorm:"type:varchar(128)"`
	CreatedAt       time.Time `gorm:"not null;autoCreateTime"`
}

func (memberInvitationAuditRow) TableName() string {
	return "listingkit_member_invitation_audits"
}

type gormAuditRepository struct {
	db *gorm.DB
}

func NewGormAuditRepository(db *gorm.DB) AuditRepository {
	return &gormAuditRepository{db: db}
}

func AutoMigrateAuditRepository(db *gorm.DB) error {
	return db.AutoMigrate(&memberInvitationAuditRow{})
}

func (r *gormAuditRepository) Record(ctx context.Context, record AuditRecord) error {
	row := memberInvitationAuditRow{
		ActorUserID:     strings.TrimSpace(record.ActorUserID),
		TenantID:        strings.TrimSpace(record.TenantID),
		Email:           strings.ToLower(strings.TrimSpace(record.Email)),
		Role:            strings.TrimSpace(record.Role),
		UserID:          strings.TrimSpace(record.UserID),
		AuthorizationID: strings.TrimSpace(record.AuthorizationID),
		Outcome:         record.Outcome,
		ErrorCode:       strings.TrimSpace(record.ErrorCode),
		CreatedAt:       time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}
