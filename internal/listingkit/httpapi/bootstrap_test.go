package httpapi

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildServiceInputFixture() BuildServiceInput {
	return BuildServiceInput{
		Config: &config.Config{},
		Repositories: BuildServiceRepositories{
			Core: CoreRepositoryBuilders{
				Task: func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
					return nil, nil, nil
				},
				Subscription: func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
					return nil, nil, nil
				},
				Asset: func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error) {
					return nil, nil, nil
				},
				Review: func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error) {
					return nil, nil, nil
				},
				StudioSession: func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
					return nil, nil, nil
				},
				UploadedImage: func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreProfile: func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreRoutingSettings: func(*config.Config, *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
					return nil, nil, nil
				},
				SheinResolutionCache: func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
					return nil, nil, nil
				},
			},
			Admin: AdminRepositoryBuilders{
				Store: func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
					return nil, nil, nil
				},
				StoreStatistics: func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
					return nil, nil, nil
				},
				ImportTask: func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
					return nil, nil, nil
				},
				FilterRule: func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProfitRule: func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				PricingRule: func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
					return nil, nil, nil
				},
				OperationStrategy: func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
					return nil, nil, nil
				},
				SensitiveWord: func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProductImportMapping: func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
					return nil, nil, nil
				},
				Category: func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
					return nil, nil, nil
				},
				ProductData: func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
					return nil, nil, nil
				},
			},
		},
		Hooks: BuildServiceHooks{
			SheinPricingPolicyBuilder: func(*config.Config) sheinpub.PricingPolicy { return sheinpub.PricingPolicy{} },
			ImageUploadStoreBuilder:   func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore { return nil },
			LegacyTenantResolverConfigurator: func(*config.Config, *logrus.Logger) (func() error, error) {
				return nil, nil
			},
			SheinCategoryLLMClientBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return nil
			},
			SheinSaleAttributeLLMBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
				return nil
			},
			SheinCategoryResolverBuilder: func(openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
				return nil
			},
			SheinAttributeResolverBuilder: func(openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
				return nil
			},
			SheinSaleAttributeResolverBuilder: func(openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
				return nil
			},
			SheinProductAPIBuilderFactory: func() sheinpub.ProductAPIBuilder {
				return nil
			},
			SheinImageAPIBuilderFactory: func() sheinpub.ImageAPIBuilder {
				return nil
			},
			SheinTranslateAPIBuilderFactory: func() sheinpub.TranslateAPIBuilder {
				return nil
			},
			SheinAPIClientFactoryBuilder: func() listingkit.SheinAPIClientFactory {
				return nil
			},
			StudioImageGeneratorBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
				return nil
			},
			DefaultSheinStoreIDResolver: func([]int64) int64 { return 0 },
			ConfigureZitadelAuth:        func(config.ListingKitZitadelConfig) {},
			ConfigureAuthorization:      func([]string, []string) error { return nil },
		},
	}
}

func TestBuildServiceClosesAcquiredResourcesWhenBuilderFails(t *testing.T) {
	t.Parallel()

	var closed []string
	input := buildServiceInputFixture()
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "task")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return nil, nil, errors.New("store repo boom")
	}

	if _, err := BuildService(input); err == nil {
		t.Fatal("expected builder failure")
	}
	if !reflect.DeepEqual(closed, []string{"task"}) {
		t.Fatalf("closed = %v, want [task]", closed)
	}
}

func TestBuildServiceValidateRequiresGroupedBuildersAndHooks(t *testing.T) {
	t.Parallel()

	input := buildServiceInputFixture()
	input.Repositories.Core.Task = nil

	_, err := BuildService(input)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if !strings.Contains(err.Error(), "core repository builder task is required") {
		t.Fatalf("err = %v, want missing task builder error", err)
	}
}

func TestBuildServiceClosesResourcesInReverseOrderOnFailure(t *testing.T) {
	t.Parallel()

	var closed []string
	input := buildServiceInputFixture()
	input.Repositories.Core.Task = func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "task")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return nil, []func() error{
			func() error {
				closed = append(closed, "store")
				return nil
			},
		}, nil
	}
	input.Repositories.Admin.StoreStatistics = func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
		return nil, nil, errors.New("stats repo boom")
	}

	if _, err := BuildService(input); err == nil {
		t.Fatal("expected builder failure")
	}
	if !reflect.DeepEqual(closed, []string{"store", "task"}) {
		t.Fatalf("closed = %v, want [store task]", closed)
	}
}
