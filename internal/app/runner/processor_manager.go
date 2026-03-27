package runner

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
)

func (s *processorServiceImpl) startProcessors(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("starting processors")

	startedAny := false
	for _, module := range s.processorModules() {
		if !module.enabled(cfg) {
			s.logger.Infof("%s processor disabled, skipping", strings.ToUpper(module.name))
			continue
		}
		if err := module.start(ctx, s, cfg); err != nil {
			return fmt.Errorf("start %s processor: %w", strings.ToUpper(module.name), err)
		}
		startedAny = true
	}

	if !startedAny {
		s.logger.Warn("all processors disabled, system will run idle")
	}

	s.logger.Info("processor startup flow complete")
	return nil
}

func (s *processorServiceImpl) stopAllProcessors(ctx context.Context) {
	for _, module := range s.processorModules() {
		processor := module.get(s)
		if processor == nil {
			continue
		}
		processor.Close(ctx)
		s.logger.Infof("%s processor stopped", strings.ToUpper(module.name))
	}

	s.logger.Info("all processors stopped")
}
