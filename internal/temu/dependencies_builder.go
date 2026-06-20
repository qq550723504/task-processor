package temu

import (
	"context"

	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/state"
)

type dependencyRuntime interface {
	managementRuntime
	state.DailyCountClientProvider
}

func BuildDependencies(
	ctx context.Context,
	runtime dependencyRuntime,
	productFetcher appfetcher.ProductFetcher,
	rabbitmqClient *rabbitmq.Client,
) Dependencies {
	mem := state.NewMemoryManager(ctx, runtime)
	if runtime != nil {
		mem.ShopPauseManager.SetStoreClient(runtime.GetStoreClient())
	}

	return Dependencies{
		ManagementRuntime: runtime,
		TaskStatusRuntime: runtime,
		MemoryManager:     mem,
		ProductFetcher:    productFetcher,
		RabbitMQClient:    rabbitmqClient,
	}
}
