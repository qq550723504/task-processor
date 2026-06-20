package pipeline

import (
	"context"

	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"
)

type dependencyRuntime interface {
	managementRuntime
	taskstatus.RuntimeWithTaskRPC
	state.DailyCountClientProvider
	GetStoreClient() *management.StoreAPIClient
	GetImageDownloader() *management.ImageDownloader
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

	var imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
	if runtime != nil {
		imageDownloader = runtime.GetImageDownloader()
	}

	return Dependencies{
		ManagementClient:  runtime,
		TaskStatusRuntime: runtime,
		MemoryManager:     mem,
		ImageDownloader:   imageDownloader,
		ProductFetcher:    productFetcher,
		RabbitMQClient:    rabbitmqClient,
	}
}
