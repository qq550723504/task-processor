package api

import (
	"errors"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func newHandlerWithDefaults() *handler {
	return &handler{}
}

func (h *handler) attachCoreServices(service handlerCoreService) {
	if service == nil {
		return
	}
	h.taskLifecycleService = service
	h.uploadedImageService = service
}

func (h *handler) attachOptionalServices(service any) {
	if service == nil {
		return
	}
	if adminService, ok := any(service).(storeAdminHandlerService); ok {
		h.storeAdminService = adminService
	}
	if settingsService, ok := any(service).(settingsHandlerService); ok && h.settingsService == nil {
		h.attachSettingsService(settingsService)
	}
	if retryService, ok := service.(childTaskRetryService); ok {
		h.childTaskRetryService = retryService
	}
	if repairService, ok := service.(taskSDSRepairService); ok {
		h.taskSDSRepairService = repairService
	}
	if recoveryService, ok := any(service).(listingkit.TaskRecoveryService); ok {
		h.taskRecoveryService = recoveryService
	}
	if requeueService, ok := any(service).(listingkit.TaskRequeueService); ok {
		h.taskRequeueService = requeueService
	}
	if warmService, ok := service.(listingkit.SDSBaselineWarmService); ok {
		h.sdsBaselineWarmService = warmService
	}
}

func (h *handler) attachSettingsService(service settingsHandlerService) {
	if service == nil {
		return
	}
	h.settingsService = newSettingsService(service)
}

func (h *handler) applyOptions(opts []HandlerOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
}

func (h *handler) finalize() error {
	if h.initErr != nil {
		return h.initErr
	}
	if h.taskLifecycleService == nil {
		return errors.New("task lifecycle service is not configured")
	}
	if h.uploadedImageService == nil {
		return errors.New("uploaded image service is not configured")
	}
	return nil
}

func NewHandler(service HandlerService, opts ...HandlerOption) (*handler, error) {
	h := newHandlerWithDefaults()
	h.attachCoreServices(service)
	h.attachOptionalServices(service)
	h.applyOptions(opts)
	if err := h.finalize(); err != nil {
		return nil, err
	}
	return h, nil
}

// NewPlatformSubscriptionHandler assembles the existing platform subscription
// owner routes without requiring the product task lifecycle runtime. This is
// used by the account-center composition, where commercial ownership is
// mounted separately from product execution.
func NewPlatformSubscriptionHandler(service *listingsubscription.Service, opts ...HandlerOption) (*handler, error) {
	if service == nil {
		return nil, errors.New("subscription service is not configured")
	}
	h := newHandlerWithDefaults()
	h.applyOptions(append([]HandlerOption{WithSubscriptionService(service)}, opts...))
	if h.subscriptionService == nil || h.subscriptionHandler == nil {
		return nil, errors.New("platform subscription handler is not configured")
	}
	return h, nil
}
