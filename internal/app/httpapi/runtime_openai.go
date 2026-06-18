package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type openAIRuntimeDeps struct {
	openaiMgr         *openaiclient.Manager
	aiCredentialStore *openaiclient.GormCredentialResolver
	closers           []func() error
}

func buildOpenAIRuntimeDeps(cfg *config.Config, logger *logrus.Logger) (*openAIRuntimeDeps, error) {
	openaiMgr, err := newOpenAIManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI manager: %w", err)
	}

	deps := &openAIRuntimeDeps{
		openaiMgr: openaiMgr,
		closers:   make([]func() error, 0),
	}
	if cfg.Database == nil {
		return deps, nil
	}

	credentialResolver, closer, err := newDBOpenAICredentialResolver(cfg.Database, logger)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI credential resolver: %w", err)
	}
	deps.aiCredentialStore = credentialResolver
	openaiMgr.SetConfigResolver(credentialResolver)
	if closer != nil {
		deps.closers = append(deps.closers, closer)
	}
	return deps, nil
}
