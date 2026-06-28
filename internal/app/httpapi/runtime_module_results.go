package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	prompt "task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/taskrpcapi"
)

type promptModuleBuilder func(store prompt.TenantPromptStore) *promptmgmtapi.BuildResult

type sdsModuleBuilder func(logger *logrus.Logger, cfg *config.Config) *sdshttpapi.BuildResult

type taskRPCModuleBuilder func(localStatusProvider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error)

type supportFeatureSet struct {
	promptModule  *promptmgmtapi.BuildResult
	taskRPCResult *taskrpcapi.BuildResult
	sdsModule     *sdshttpapi.BuildResult
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

func buildPromptModuleResult(store prompt.TenantPromptStore) *promptmgmtapi.BuildResult {
	return promptmgmtapi.BuildModule(store)
}

func buildSDSModuleResult(logger *logrus.Logger, cfg *config.Config) *sdshttpapi.BuildResult {
	return sdshttpapi.BuildModule(logger, cfg)
}

func buildTaskRPCModuleResult(localStatusProvider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error) {
	return taskrpcapi.BuildModule(localStatusProvider)
}
