package httpapi

import (
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/taskrpcapi"
)

type promptModuleResult = promptmgmtapi.BuildResult

type sdsModuleResult = sdshttpapi.BuildResult

type taskRPCModuleResult = taskrpcapi.BuildResult
