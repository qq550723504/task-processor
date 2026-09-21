package httpapi

import (
	"errors"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	listingkitapi "task-processor/internal/listingkit/api"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/listingsubscription"
)

func buildPlatformSubscriptionModule(db *gorm.DB, cfg *config.Config) (kernelmodule.Module, error) {
	if db == nil || cfg == nil {
		return nil, errors.New("platform subscription owner dependencies unavailable")
	}
	service, err := listingsubscription.NewService(listingsubscription.NewGormRepository(db))
	if err != nil {
		return nil, err
	}
	handler, err := listingkitapi.NewPlatformSubscriptionHandler(
		service,
		listingkitapi.WithPlatformSubscriptionAccess(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles),
	)
	if err != nil {
		return nil, err
	}
	return listingkithttpapi.NewPlatformAdminModule(handler), nil
}
