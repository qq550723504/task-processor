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

type taskRPCModuleBuilder func(localStatusProvider taskrpcapi.LocalStatusProvider) (*taskRPCModuleResult, error)

type supportFeatureSet struct {
	promptModule  *promptModuleResult
	taskRPCResult *taskRPCModuleResult
	sdsModule     *sdsModuleResult
}

type supportFeatureBuilder struct {
	buildPrompt  promptModuleBuilder
	buildTaskRPC taskRPCModuleBuilder
	buildSDS     sdsModuleBuilder
}

func (b supportFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps, composition httpFeatureComposition) (supportFeatureSet, error) {
	var features supportFeatureSet

	features.promptModule = b.buildPrompt(deps.shared.tenantPromptStore)
	composition.promptModule = features.promptModule

	runtimeBundle, err := buildRuntimeBundleFromModules(deps.shared.cfg, composition.runtimeModules())
	if err != nil {
		return features, err
	}

	taskRPCResult, err := b.buildTaskRPC(runtimeBundle.localTaskHealthProvider())
	if err != nil {
		return features, err
	}
	features.taskRPCResult = taskRPCResult

	features.sdsModule = b.buildSDS(logger, deps.shared.cfg)

	return features, nil
}

func buildPromptModuleResult(store prompt.TenantPromptStore) *promptModuleResult {
	return promptmgmtapi.BuildModule(store)
}

func buildSDSModuleResult(logger *logrus.Logger, cfg *config.Config) *sdsModuleResult {
	return sdshttpapi.BuildModule(logger, cfg)
}

func buildTaskRPCModuleResult(localStatusProvider taskrpcapi.LocalStatusProvider) (*taskRPCModuleResult, error) {
	return taskrpcapi.BuildModule(localStatusProvider)
}
