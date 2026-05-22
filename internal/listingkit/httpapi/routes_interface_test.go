package httpapi

import "github.com/gin-gonic/gin"

type settingsOnlyRouteHandler struct{}

func (settingsOnlyRouteHandler) ListSettingsNamespaces(c *gin.Context)      {}
func (settingsOnlyRouteHandler) GetSettingsNamespaceSchema(c *gin.Context)  {}
func (settingsOnlyRouteHandler) GetSettingsNamespace(c *gin.Context)        {}
func (settingsOnlyRouteHandler) UpdateSettingsNamespace(c *gin.Context)     {}
func (settingsOnlyRouteHandler) ListSheinStoreProfiles(c *gin.Context)      {}
func (settingsOnlyRouteHandler) UpsertSheinStoreProfile(c *gin.Context)     {}
func (settingsOnlyRouteHandler) DeleteSheinStoreProfile(c *gin.Context)     {}
func (settingsOnlyRouteHandler) GetSheinStoreRoutingSettings(c *gin.Context) {}
func (settingsOnlyRouteHandler) UpdateSheinStoreRoutingSettings(c *gin.Context) {
}
func (settingsOnlyRouteHandler) GetSheinSettings(c *gin.Context)     {}
func (settingsOnlyRouteHandler) UpdateSheinSettings(c *gin.Context)  {}
func (settingsOnlyRouteHandler) GetAIClientSettings(c *gin.Context)  {}
func (settingsOnlyRouteHandler) UpdateAIClientSettings(c *gin.Context) {}

type taskOnlyRouteHandler struct{}

func (taskOnlyRouteHandler) GenerateListingKit(c *gin.Context)                {}
func (taskOnlyRouteHandler) ListTasks(c *gin.Context)                         {}
func (taskOnlyRouteHandler) UploadListingKitImages(c *gin.Context)            {}
func (taskOnlyRouteHandler) GetUploadedListingKitImage(c *gin.Context)        {}
func (taskOnlyRouteHandler) DeleteUploadedListingKitImage(c *gin.Context)     {}
func (taskOnlyRouteHandler) GenerateStudioDesigns(c *gin.Context)             {}
func (taskOnlyRouteHandler) GenerateStudioProductImages(c *gin.Context)       {}
func (taskOnlyRouteHandler) StartStudioAsyncJob(c *gin.Context)               {}
func (taskOnlyRouteHandler) GetStudioAsyncJob(c *gin.Context)                 {}
func (taskOnlyRouteHandler) RegenerateSheinDataImage(c *gin.Context)          {}
func (taskOnlyRouteHandler) GetTaskResult(c *gin.Context)                     {}
func (taskOnlyRouteHandler) GetTaskPreview(c *gin.Context)                    {}
func (taskOnlyRouteHandler) GetTaskGenerationTasks(c *gin.Context)            {}
func (taskOnlyRouteHandler) GetTaskGenerationQueue(c *gin.Context)            {}
func (taskOnlyRouteHandler) GetTaskGenerationReviewSession(c *gin.Context)    {}
func (taskOnlyRouteHandler) GetTaskGenerationReviewPreview(c *gin.Context)    {}
func (taskOnlyRouteHandler) DispatchTaskGenerationNavigation(c *gin.Context)  {}
func (taskOnlyRouteHandler) RetryTaskGenerationTasks(c *gin.Context)          {}
func (taskOnlyRouteHandler) RetryTaskChildTask(c *gin.Context)                {}
func (taskOnlyRouteHandler) ExecuteTaskGenerationAction(c *gin.Context)       {}
func (taskOnlyRouteHandler) GetTaskRevisionHistory(c *gin.Context)            {}
func (taskOnlyRouteHandler) GetTaskRevisionHistoryDetail(c *gin.Context)      {}
func (taskOnlyRouteHandler) GetTaskExport(c *gin.Context)                     {}
func (taskOnlyRouteHandler) ApplyTaskRevision(c *gin.Context)                 {}
func (taskOnlyRouteHandler) ValidateTaskRevision(c *gin.Context)              {}
func (taskOnlyRouteHandler) SubmitTask(c *gin.Context)                        {}
func (taskOnlyRouteHandler) RefreshSubmissionStatus(c *gin.Context)           {}
func (taskOnlyRouteHandler) PreviewSheinPrice(c *gin.Context)                 {}
func (taskOnlyRouteHandler) SearchSheinCategories(c *gin.Context)             {}
func (taskOnlyRouteHandler) UpdateSheinFinalDraft(c *gin.Context)             {}
func (taskOnlyRouteHandler) GetSubmissionEvents(c *gin.Context)               {}
func (taskOnlyRouteHandler) ClearSheinResolutionCache(c *gin.Context)         {}

var _ SettingsRouteHandler = settingsOnlyRouteHandler{}
var _ TaskRouteHandler = taskOnlyRouteHandler{}
