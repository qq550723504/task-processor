package httpapi

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	managementclient "task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
)

type localManagementPromotionStrategyProvider struct {
	client *managementclient.OperationStrategyClient
}

func (p localManagementPromotionStrategyProvider) GetPromotionStrategy(_ context.Context, storeID int64, _ string) (*managementapi.OperationStrategyDTO, error) {
	if p.client == nil {
		return nil, fmt.Errorf("SHEIN promotion strategy client is not configured")
	}
	return p.client.GetOperationStrategyByStoreId(storeID)
}

func buildSheinPromotionStrategyProvider(input BuildServiceInput, closers *closerStack) (localManagementPromotionStrategyProvider, error) {
	var managementCfg *config.ManagementConfig
	if input.Config != nil {
		managementCfg = &input.Config.Management
	}
	clientManager := managementclient.NewClientManager(managementCfg)

	var dbCfg *config.DatabaseConfig
	var redisCfg *config.RedisConfig
	if input.Config != nil {
		dbCfg = input.Config.Database
		redisCfg = input.Config.Redis
	}
	localProvider, err := managementclient.NewLocalDataProvider(dbCfg, redisCfg)
	if err != nil {
		return localManagementPromotionStrategyProvider{}, fmt.Errorf("create local management data provider: %w", err)
	}
	if localProvider != nil {
		clientManager.SetLocalDataProvider(localProvider)
		closers.Add(localProvider.Close)
	}
	return localManagementPromotionStrategyProvider{client: clientManager.GetOperationStrategyClient()}, nil
}
