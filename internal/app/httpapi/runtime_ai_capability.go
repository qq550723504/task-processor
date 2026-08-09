package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
)

type aiCapabilityRuntimeDeps struct {
	invocationRecorder aicapability.InvocationRecorder
	asyncJobStore      aicapability.AsyncJobBindingStore
	closers            []func() error
}

func buildAICapabilityRuntimeDeps(cfg *config.Config, logger *logrus.Logger) (*aiCapabilityRuntimeDeps, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AI capability configuration is nil")
	}
	mode, err := aicapability.ParseRoutingMode(cfg.AICapability.StudioImageRoutingMode)
	if err != nil {
		return nil, fmt.Errorf("parse AI capability Studio image routing mode: %w", err)
	}
	deps := &aiCapabilityRuntimeDeps{}
	if mode == aicapability.RoutingModeLegacy && !cfg.AICapability.ProductImageSceneEnabled {
		return deps, nil
	}
	if cfg.Database == nil {
		return nil, fmt.Errorf("AI capability invocation ledger requires database configuration for %s mode", mode)
	}
	recorder, asyncJobStore, closer, err := newDBAICapabilityStores(cfg.Database, logger)
	if err != nil {
		return nil, fmt.Errorf("create AI capability invocation recorder: %w", err)
	}
	deps.invocationRecorder = recorder
	deps.asyncJobStore = asyncJobStore
	if closer != nil {
		deps.closers = append(deps.closers, closer)
	}
	return deps, nil
}
