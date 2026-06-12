package httpapi

import "task-processor/internal/listingkit"

var _ listingkit.TaskLifecycleService = (moduleService)(nil)
var _ listingkit.GenerationTaskService = (moduleService)(nil)
var _ listingkit.StudioMediaService = (moduleService)(nil)
