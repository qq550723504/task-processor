package platformbase

import (
	"errors"
	"testing"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/platform/queue/rabbitmq"

	"github.com/stretchr/testify/require"
)

func TestBaseFactoryBuildProductFetcherRequiresInjectedBuilder(t *testing.T) {
	factory := NewBaseFactory(BaseFactoryConfig{Platform: "TEST"})

	productFetcher, err := factory.BuildProductFetcher(nil)
	if productFetcher != nil {
		t.Fatalf("BuildProductFetcher() fetcher = %#v, want nil", productFetcher)
	}
	if !errors.Is(err, ErrProductFetcherBuilderRequired) {
		t.Fatalf("BuildProductFetcher() error = %v, want ErrProductFetcherBuilderRequired", err)
	}
}

func TestBaseFactoryBuildProductFetcherUsesNarrowInjectedBuilder(t *testing.T) {
	var gotAmazonConfig *config.AmazonConfig
	var gotRabbitMQClient *rabbitmq.Client
	wantAmazonConfig := &config.AmazonConfig{}
	wantRabbitMQClient := &rabbitmq.Client{}
	factory := NewBaseFactory(BaseFactoryConfig{
		Platform:     "TEST",
		AmazonConfig: wantAmazonConfig,
		FetcherBuilder: ProductFetcherBuilderFunc(func(
			amazonConfig *config.AmazonConfig,
			rabbitmqClient *rabbitmq.Client,
		) (appfetcher.ProductFetcher, error) {
			gotAmazonConfig = amazonConfig
			gotRabbitMQClient = rabbitmqClient
			return nil, nil
		}),
	})

	_, err := factory.BuildProductFetcher(wantRabbitMQClient)
	if err != nil {
		t.Fatalf("BuildProductFetcher() error = %v", err)
	}
	if gotAmazonConfig != wantAmazonConfig {
		t.Fatalf("builder amazon config = %p, want %p", gotAmazonConfig, wantAmazonConfig)
	}
	if gotRabbitMQClient != wantRabbitMQClient {
		t.Fatalf("builder RabbitMQ client = %p, want %p", gotRabbitMQClient, wantRabbitMQClient)
	}
}

func TestResolvePlatformFetcherType(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "local"
	cfg.Platforms.Temu.FetchMode = "remote-api"

	fetcherType, err := ResolvePlatformFetcherType(cfg, "shein")
	require.NoError(t, err)
	require.Equal(t, appfetcher.LocalFetcher, fetcherType)

	fetcherType, err = ResolvePlatformFetcherType(cfg, "temu")
	require.NoError(t, err)
	require.Equal(t, appfetcher.RemoteAPIFetcher, fetcherType)

	fetcherType, err = ResolvePlatformFetcherType(cfg, "")
	require.NoError(t, err)
	require.Equal(t, appfetcher.FetcherType(""), fetcherType)
}

func TestResolvePlatformFetcherTypeRejectsInvalidMode(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefaultConfig()
	cfg.Platforms.Shein.FetchMode = "invalid"

	_, err := ResolvePlatformFetcherType(cfg, "shein")
	require.Error(t, err)
}
