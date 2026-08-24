package memberinvite

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Outcome string

const (
	OutcomeSucceeded        Outcome = "succeeded"
	OutcomeFailed           Outcome = "failed"
	OutcomeIncomplete       Outcome = "incomplete"
	auditMigrationBatchSize         = 200
)

// AuditRecord is the non-sensitive, durable record of one member invitation attempt.
type AuditRecord struct {
	ActorUserID     string
	TenantID        string
	Email           string
	Phone           string
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
	Phone           string    `gorm:"type:varchar(32)"`
	DeliveryMode    string    `gorm:"type:varchar(32);not null;default:''"`
	Contact         string    `gorm:"type:varchar(512);not null;default:''"`
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
	if err := db.AutoMigrate(&memberInvitationAuditRow{}); err != nil {
		return err
	}
	var rows []memberInvitationAuditRow
	return db.Where("delivery_mode = ? OR contact = ?", "", "").FindInBatches(&rows, auditMigrationBatchSize, func(tx *gorm.DB, _ int) error {
		for _, row := range rows {
			delivery := DeliveryMetadataFor(row.Email, "")
			if row.DeliveryMode != "" {
				delivery.Mode = row.DeliveryMode
			}
			if row.Contact != "" {
				delivery.Contact = row.Contact
			}
			if err := tx.Model(&memberInvitationAuditRow{}).Where("id = ?", row.ID).Updates(map[string]any{
				"email":         delivery.Email,
				"delivery_mode": delivery.Mode,
				"contact":       delivery.Contact,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}).Error
}

func (r *gormAuditRepository) Record(ctx context.Context, record AuditRecord) error {
	delivery := DeliveryMetadataFor(record.Email, record.Phone)
	row := memberInvitationAuditRow{
		ActorUserID:     strings.TrimSpace(record.ActorUserID),
		TenantID:        strings.TrimSpace(record.TenantID),
		Email:           delivery.Email,
		Phone:           delivery.Phone,
		DeliveryMode:    delivery.Mode,
		Contact:         delivery.Contact,
		Role:            strings.TrimSpace(record.Role),
		UserID:          strings.TrimSpace(record.UserID),
		AuthorizationID: strings.TrimSpace(record.AuthorizationID),
		Outcome:         record.Outcome,
		ErrorCode:       strings.TrimSpace(record.ErrorCode),
		CreatedAt:       time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}
