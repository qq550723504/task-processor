package consumer

import (
	apptask "task-processor/internal/app/task"
	"task-processor/internal/infra/clients/management/api"
)

func (s *RabbitMQService) applyComponentDependencies(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *apptask.DeduplicationManager,
) {
	if resultReporter != nil {
		s.resultReporter = resultReporter
	}
	if storeAPI != nil {
		s.storeAPI = storeAPI
	}
	if ownedStores != nil {
		s.ownedStores = append([]int64(nil), ownedStores...)
	}
	if deduplicator != nil {
		s.deduplicator = deduplicator
	}
	s.useStoreQueues = s.config.Node.UseStoreQueues || s.storeAssignmentProvider != nil
}

func (s *RabbitMQService) syncProcessorRegistryComponents() {
	s.processorRegistry.UpdateComponents(
		s.resultReporter,
		s.storeAPI,
		append([]int64(nil), s.ownedStores...),
		&s.useStoreQueues,
		s.deduplicator,
	)
}
