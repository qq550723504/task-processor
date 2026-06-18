package sheinsync

import managementapi "task-processor/internal/infra/clients/management/api"

type SheinPromotionStrategy struct {
	managementStrategy *managementapi.OperationStrategyDTO
}

func NewSheinPromotionStrategy(strategy *managementapi.OperationStrategyDTO) *SheinPromotionStrategy {
	if strategy == nil {
		return nil
	}
	return &SheinPromotionStrategy{managementStrategy: strategy}
}

func (s *SheinPromotionStrategy) managementOperationStrategy() *managementapi.OperationStrategyDTO {
	if s == nil {
		return nil
	}
	return s.managementStrategy
}
