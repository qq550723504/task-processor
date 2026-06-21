package api

import (
	"errors"

	"task-processor/internal/listingkit"
)

func newHandlerWithDefaults(studioAsyncJobs studioAsyncJobStoreService) *handler {
	return &handler{
		studioBatchRunService: nil,
		studioAsyncJobs:       studioAsyncJobs,
	}
}

func (h *handler) attachCoreServices(service handlerCoreService) {
	if service == nil {
		return
	}
	h.taskLifecycleService = service
	h.generationTaskService = service
	h.studioMediaService = service
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
	if recoveryService, ok := any(service).(listingkit.TaskRecoveryService); ok {
		h.taskRecoveryService = recoveryService
	}
	if requeueService, ok := any(service).(listingkit.TaskRequeueService); ok {
		h.taskRequeueService = requeueService
	}
	if sessionService, ok := service.(studioSessionAsyncJobService); ok {
		h.studioSessionService = sessionService
	}
	if batchRunService, ok := service.(studioBatchRunHandlerService); ok {
		h.studioBatchRunService = batchRunService
	}
	if warmService, ok := service.(listingkit.SDSBaselineWarmService); ok {
		h.sdsBaselineWarmService = warmService
	}
	if deleteService, ok := service.(uploadedImageDeleteService); ok {
		h.uploadedImageDeleteService = deleteService
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
	if h.generationTaskService == nil {
		return errors.New("generation task service is not configured")
	}
	if h.studioMediaService == nil {
		return errors.New("studio media service is not configured")
	}
	return nil
}

func NewHandler(service HandlerService, opts ...HandlerOption) (*handler, error) {
	studioAsyncJobs, err := newStudioAsyncJobStore(nil)
	if err != nil {
		return nil, err
	}
	h := newHandlerWithDefaults(studioAsyncJobs)
	h.attachCoreServices(service)
	h.attachOptionalServices(service)
	h.applyOptions(opts)
	if err := h.finalize(); err != nil {
		return nil, err
	}
	return h, nil
}
