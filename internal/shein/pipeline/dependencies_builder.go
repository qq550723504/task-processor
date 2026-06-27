package pipeline

import (
	"context"

	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"
)

type dependencyRuntime interface {
	runtimeRepository
	taskstatus.RuntimeWithTaskRPC
	state.DailyCountClientProvider
	GetStoreAPI() listingadmin.StoreAPI
	GetImageDownloader() interface {
		DownloadImage(url string) ([]byte, error)
	}
}

func BuildDependencies(
	ctx context.Context,
	runtime dependencyRuntime,
	productFetcher appfetcher.ProductFetcher,
	rabbitmqClient *rabbitmq.Client,
) Dependencies {
	mem := state.NewMemoryManager(ctx, runtime)
	if runtime != nil {
		mem.ShopPauseManager.SetStoreClient(runtime.GetStoreAPI())
	}

	var imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
	if runtime != nil {
		imageDownloader = runtime.GetImageDownloader()
	}

	return Dependencies{
		RuntimeRepository: runtime,
		TaskStatusRuntime: runtime,
		MemoryManager:     mem,
		ImageDownloader:   imageDownloader,
		ProductFetcher:    productFetcher,
		RabbitMQClient:    rabbitmqClient,
	}
}
