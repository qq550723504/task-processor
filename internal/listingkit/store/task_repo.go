package store

import (
	"errors"

	"gorm.io/gorm"

	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/listingkit"
)

type taskRepository struct {
	db           *gorm.DB
	tenantAdmins listingtask.TenantAdminChecker
}

func NewTaskRepository(db *gorm.DB) listingkit.Repository {
	return &taskRepository{db: db, tenantAdmins: authz.DefaultListingKitAuthorizer()}
}

// NewTaskRepositoryWithTenantAdminChecker lets a composition root share one
// configured authorization instance across permission preflight and storage
// owner-scope decisions.
func NewTaskRepositoryWithTenantAdminChecker(db *gorm.DB, tenantAdmins listingtask.TenantAdminChecker) (listingkit.Repository, error) {
	if tenantAdmins == nil {
		return nil, errors.New("tenant admin checker is nil")
	}
	return &taskRepository{db: db, tenantAdmins: tenantAdmins}, nil
}

// NewTaskResultTransactionRepository exposes only the canonical locked task
// result mutation boundary to runtime adapters such as image-agent approval.
func NewTaskResultTransactionRepository(db *gorm.DB) listingkit.TaskResultTransactionRepository {
	return &taskRepository{db: db}
}
