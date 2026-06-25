package consumer

import (
	apptask "task-processor/internal/app/task"
	api "task-processor/internal/ports/managementapi"
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
	if deduplicator != nil {
		s.deduplicator = deduplicator
	}
	s.applyRoutingState(serviceRoutingState{
		ownedStores:    ownedStores,
		ownedBuckets:   s.ownedBuckets,
		useStoreQueues: s.config.Node.UseStoreQueues || s.storeAssignmentProvider != nil,
	})
}

func (s *RabbitMQService) syncProcessorRegistryComponents() {
	routingState := s.routingStateSnapshotLocked()
	s.processorRegistry.UpdateComponents(
		s.resultReporter,
		s.storeAPI,
		routingState.ownedStores,
		&routingState.useStoreQueues,
		s.deduplicator,
	)
}
