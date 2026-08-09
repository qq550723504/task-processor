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
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		email := maskEmail(row.Email)
		mode := row.DeliveryMode
		if mode == "" {
			mode = "email"
		}
		contact := row.Contact
		if contact == "" {
			contact = email
		}
		if err := db.Model(&memberInvitationAuditRow{}).Where("id = ?", row.ID).Updates(map[string]any{
			"email":         email,
			"delivery_mode": mode,
			"contact":       contact,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *gormAuditRepository) Record(ctx context.Context, record AuditRecord) error {
	email := maskEmail(record.Email)
	phone := maskPhone(record.Phone)
	deliveryMode := "email"
	contact := email
	if phone != "" {
		deliveryMode = "email_phone"
		contact += "," + phone
	}
	row := memberInvitationAuditRow{
		ActorUserID:     strings.TrimSpace(record.ActorUserID),
		TenantID:        strings.TrimSpace(record.TenantID),
		Email:           email,
		Phone:           phone,
		DeliveryMode:    deliveryMode,
		Contact:         contact,
		Role:            strings.TrimSpace(record.Role),
		UserID:          strings.TrimSpace(record.UserID),
		AuthorizationID: strings.TrimSpace(record.AuthorizationID),
		Outcome:         record.Outcome,
		ErrorCode:       strings.TrimSpace(record.ErrorCode),
		CreatedAt:       time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func maskEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	local := email[:at]
	if len(local) == 1 {
		return "*" + email[at:]
	}
	return local[:1] + "***" + email[at:]
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if len(phone) <= 5 {
		return "***"
	}
	return phone[:3] + "***" + phone[len(phone)-4:]
}
