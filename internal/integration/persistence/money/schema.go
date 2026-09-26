package money

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/ledger/money"
)

// Install is the explicit greenfield schema entry point for the canonical
// money owner. It is never called by a serving handler.
func Install(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return money.ErrInvalid
	}
	return AutoMigrate(db)
}
