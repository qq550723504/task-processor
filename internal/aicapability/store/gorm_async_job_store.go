package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/aicapability"
)

type GormAsyncJobBindingStore struct {
	db *gorm.DB
}

func NewGormAsyncJobBindingStore(db *gorm.DB) *GormAsyncJobBindingStore {
	return &GormAsyncJobBindingStore{db: db}
}

func AutoMigrateAsyncJobBindings(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ai async job binding database is nil")
	}
	return db.AutoMigrate(&asyncJobRow{})
}

func (s *GormAsyncJobBindingStore) PutAsyncJobBinding(ctx context.Context, binding aicapability.AsyncJobBinding) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("ai async job binding database is nil")
	}
	if err := aicapability.ValidateAsyncJobBinding(binding); err != nil {
		return err
	}
	binding = normalizeAsyncJobBinding(binding)
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "job_id"}},
		DoNothing: true,
	}).Create(asyncJobRowFromBinding(binding))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var existing asyncJobRow
	if err := s.db.WithContext(ctx).Where("job_id = ?", binding.JobID).First(&existing).Error; err != nil {
		return err
	}
	if sameAsyncJobRoute(existing, binding) {
		return nil
	}
	return fmt.Errorf("%w: job_id %q", aicapability.ErrAsyncJobBindingConflict, binding.JobID)
}

func (s *GormAsyncJobBindingStore) GetAsyncJobBinding(ctx context.Context, jobID string) (aicapability.AsyncJobBinding, error) {
	if s == nil || s.db == nil {
		return aicapability.AsyncJobBinding{}, fmt.Errorf("ai async job binding database is nil")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return aicapability.AsyncJobBinding{}, aicapability.ErrAsyncJobBindingInvalid
	}
	var row asyncJobRow
	if err := s.db.WithContext(ctx).Where("job_id = ?", jobID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return aicapability.AsyncJobBinding{}, aicapability.ErrAsyncJobBindingNotFound
		}
		return aicapability.AsyncJobBinding{}, err
	}
	return asyncJobBindingFromRow(row), nil
}

func (s *GormAsyncJobBindingStore) UpdateAsyncJobBindingStatus(ctx context.Context, jobID, status string, category aicapability.ErrorCategory) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("ai async job binding database is nil")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return aicapability.ErrAsyncJobBindingInvalid
	}
	result := s.db.WithContext(ctx).Model(&asyncJobRow{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":              strings.TrimSpace(status),
		"last_error_category": strings.TrimSpace(string(category)),
		"updated_at":          time.Now().UTC(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return aicapability.ErrAsyncJobBindingNotFound
	}
	return nil
}

type asyncJobRow struct {
	JobID                string    `gorm:"column:job_id;primaryKey;size:256"`
	TenantID             string    `gorm:"column:tenant_id;size:128;index:idx_ai_async_jobs_tenant_submitted,priority:1"`
	UserID               string    `gorm:"column:user_id;size:128"`
	BusinessTaskID       string    `gorm:"column:business_task_id;size:128"`
	TraceID              string    `gorm:"column:trace_id;size:128"`
	Capability           string    `gorm:"column:capability;size:128;index:idx_ai_async_jobs_capability_submitted,priority:1"`
	Operation            string    `gorm:"column:operation;size:128"`
	ProviderID           string    `gorm:"column:provider_id;size:128;index:idx_ai_async_jobs_provider_model,priority:1"`
	ModelID              string    `gorm:"column:model_id;size:256;index:idx_ai_async_jobs_provider_model,priority:2"`
	RoutingKey           string    `gorm:"column:routing_key;size:256"`
	CredentialReference  string    `gorm:"column:credential_reference;size:256"`
	PolicyVersion        string    `gorm:"column:policy_version;size:128"`
	ConfigurationVersion string    `gorm:"column:configuration_version;size:128"`
	SubmittedAt          time.Time `gorm:"column:submitted_at;index:idx_ai_async_jobs_tenant_submitted,priority:2;index:idx_ai_async_jobs_capability_submitted,priority:2"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
	ExpiresAt            time.Time `gorm:"column:expires_at;index"`
	Status               string    `gorm:"column:status;size:32"`
	LastErrorCategory    string    `gorm:"column:last_error_category;size:64"`
}

func (asyncJobRow) TableName() string { return "ai_async_jobs" }

func normalizeAsyncJobBinding(binding aicapability.AsyncJobBinding) aicapability.AsyncJobBinding {
	binding.JobID = strings.TrimSpace(binding.JobID)
	binding.TenantID = strings.TrimSpace(binding.TenantID)
	binding.UserID = strings.TrimSpace(binding.UserID)
	binding.BusinessTaskID = strings.TrimSpace(binding.BusinessTaskID)
	binding.TraceID = strings.TrimSpace(binding.TraceID)
	binding.Capability = aicapability.Capability(strings.TrimSpace(string(binding.Capability)))
	binding.Operation = aicapability.Operation(strings.TrimSpace(string(binding.Operation)))
	binding.ProviderID = strings.TrimSpace(binding.ProviderID)
	binding.ModelID = strings.TrimSpace(binding.ModelID)
	binding.RoutingKey = strings.TrimSpace(binding.RoutingKey)
	binding.CredentialReference = strings.TrimSpace(binding.CredentialReference)
	binding.PolicyVersion = strings.TrimSpace(binding.PolicyVersion)
	binding.ConfigurationVersion = strings.TrimSpace(binding.ConfigurationVersion)
	binding.Status = strings.TrimSpace(binding.Status)
	binding.LastErrorCategory = aicapability.ErrorCategory(strings.TrimSpace(string(binding.LastErrorCategory)))
	binding.SubmittedAt = binding.SubmittedAt.UTC()
	binding.UpdatedAt = binding.UpdatedAt.UTC()
	binding.ExpiresAt = binding.ExpiresAt.UTC()
	return binding
}

func asyncJobRowFromBinding(binding aicapability.AsyncJobBinding) asyncJobRow {
	binding = normalizeAsyncJobBinding(binding)
	updatedAt := binding.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = binding.SubmittedAt
	}
	return asyncJobRow{
		JobID: binding.JobID, TenantID: binding.TenantID, UserID: binding.UserID, BusinessTaskID: binding.BusinessTaskID, TraceID: binding.TraceID,
		Capability: string(binding.Capability), Operation: string(binding.Operation), ProviderID: binding.ProviderID, ModelID: binding.ModelID,
		RoutingKey: binding.RoutingKey, CredentialReference: binding.CredentialReference, PolicyVersion: binding.PolicyVersion, ConfigurationVersion: binding.ConfigurationVersion,
		SubmittedAt: binding.SubmittedAt, UpdatedAt: updatedAt, ExpiresAt: binding.ExpiresAt, Status: binding.Status, LastErrorCategory: string(binding.LastErrorCategory),
	}
}

func asyncJobBindingFromRow(row asyncJobRow) aicapability.AsyncJobBinding {
	return aicapability.AsyncJobBinding{
		JobID: row.JobID, TenantID: row.TenantID, UserID: row.UserID, BusinessTaskID: row.BusinessTaskID, TraceID: row.TraceID,
		Capability: aicapability.Capability(row.Capability), Operation: aicapability.Operation(row.Operation), ProviderID: row.ProviderID, ModelID: row.ModelID,
		RoutingKey: row.RoutingKey, CredentialReference: row.CredentialReference, PolicyVersion: row.PolicyVersion, ConfigurationVersion: row.ConfigurationVersion,
		SubmittedAt: row.SubmittedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), ExpiresAt: row.ExpiresAt.UTC(), Status: row.Status, LastErrorCategory: aicapability.ErrorCategory(row.LastErrorCategory),
	}
}

func sameAsyncJobRoute(row asyncJobRow, binding aicapability.AsyncJobBinding) bool {
	binding = normalizeAsyncJobBinding(binding)
	return row.JobID == binding.JobID && row.TenantID == binding.TenantID && row.UserID == binding.UserID && row.Capability == string(binding.Capability) && row.Operation == string(binding.Operation) && row.ProviderID == binding.ProviderID && row.ModelID == binding.ModelID && row.RoutingKey == binding.RoutingKey && row.CredentialReference == binding.CredentialReference && row.PolicyVersion == binding.PolicyVersion && row.ConfigurationVersion == binding.ConfigurationVersion
}
