package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	prompt "task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/taskrpcapi"
)

type promptModuleResult = promptmgmtapi.BuildResult

type sdsModuleResult = sdshttpapi.BuildResult

type taskRPCModuleResult = taskrpcapi.BuildResult

type promptModuleBuilder func(store prompt.TenantPromptStore) *promptModuleResult

type sdsModuleBuilder func(logger *logrus.Logger, cfg *config.Config) *sdsModuleResult

type taskRPCModuleBuilder func(provider taskrpcapi.ClientProvider, localStatusProvider taskrpcapi.LocalStatusProvider) (*taskRPCModuleResult, error)

func buildPromptModuleResult(store prompt.TenantPromptStore) *promptModuleResult {
	return promptmgmtapi.BuildModule(store)
}

func buildSDSModuleResult(logger *logrus.Logger, cfg *config.Config) *sdsModuleResult {
	return sdshttpapi.BuildModule(logger, cfg)
}

func buildTaskRPCModuleResult(provider taskrpcapi.ClientProvider, localStatusProvider taskrpcapi.LocalStatusProvider) (*taskRPCModuleResult, error) {
	return taskrpcapi.BuildModule(provider, localStatusProvider)
}
