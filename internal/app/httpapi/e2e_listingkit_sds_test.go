package httpapi

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productimage"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type stubE2ESDSSyncService struct{}

func (s *stubE2ESDSSyncService) SyncFromRemoteImage(ctx context.Context, input sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (s *stubE2ESDSSyncService) SyncFromLocalFile(ctx context.Context, input sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (s *stubE2ESDSSyncService) SyncFromImageRequest(ctx context.Context, input sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func (s *stubE2ESDSSyncService) SyncFromImageResult(ctx context.Context, input sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	return &sdsadapter.SyncResult{
		ImageResult: input.ImageResult,
		DesignSync: &sdsworkflow.SyncResult{
			DesignResult: &sdsdesign.PrepareSyncDesignResult{
				Page: &sdsdesign.DesignProductPage{
					Product: sdsdesign.DesignProduct{ID: input.Sync.VariantID},
				},
				Request: &sdsdesign.SyncDesignRequest{
					ProductID:        input.Sync.VariantID,
					PrototypeGroupID: 14555,
					Prototypes: []sdsdesign.SyncDesignPrototype{{
						PrototypeID: "698744758228934657",
						Layers: []sdsdesign.SyncDesignLayer{{
							LayerID: "698744758333792256",
						}},
					}},
				},
				Material: &sdsdesign.UploadedMaterial{
					Material: &sdsdesign.Material{ID: 396548287},
				},
				RenderedImageURLs: []string{
					"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
					"https://cdn.sdspod.com/out/36811/202604/rendered-gallery.jpg",
				},
			},
		},
	}, nil
}

func TestHTTPE2E_ListingKitGenerateSyncsSDSDesign(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg, err := config.LoadConfigFromFileWithoutValidation("../../../config/config-test.yaml")
	require.NoError(t, err)
	cfg.ProductImage.WorkDir = filepath.Join(t.TempDir(), "productimage")
	cfg.ProductImage.Publisher.OutputDir = filepath.Join(t.TempDir(), "published")

	llmMgr := &e2eLLMManager{client: &e2eLLMClient{}}
	understanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	require.NoError(t, err)

	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, e2eWebScraper{})
	require.NoError(t, err)

	deps := &runtimeDeps{
		cfg:           cfg,
		llmMgr:        llmMgr,
		inputParser:   inputParser,
		understanding: understanding,
		imageWorkDir:  cfg.ProductImage.WorkDir,
	}

	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return &stubE2ESDSSyncService{}, &sdsclient.AuthState{AccessToken: "test-token"}, nil
	}

	features, err := newListingKitFeatureBuilder().build(logger, deps, listingKitFeatureBuildOptions{
		includeImage:      true,
		includeListingKit: true,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pools := []worker.WorkerPool{features.productModule.Pool, features.imageModule.Pool, features.listingKitModule.Pool}
	for _, pool := range pools {
		pool.Start(ctx)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		for _, pool := range pools {
			pool.Stop(stopCtx)
		}
		for i := len(deps.closers) - 1; i >= 0; i-- {
			require.NoError(t, deps.closers[i]())
		}
	}()

	routerServer := buildHTTPServer(0, features.productModule.Handler, features.imageModule.Handler, nil, features.listingKitModule.Handler, nil)
	testServer := httptest.NewServer(routerServer.Handler)
	defer testServer.Close()
	enableListingKitSubscriptionModule(t, testServer.Client(), testServer.URL, "studio")

	imageServer := newE2EImageServer()
	defer imageServer.Close()
	imageURL := imageServer.URL + "/product.png"

	taskID := createTaskViaAPI[map[string]any](t, testServer.Client(), testServer.URL+"/api/v1/listing-kits/generate", map[string]any{
		"text":       "高品质蓝牙耳机，支持主动降噪、蓝牙 5.3、30 小时续航和舒适佩戴。",
		"image_urls": []string{imageURL},
		"platforms":  []string{"amazon"},
		"options": map[string]any{
			"process_images": true,
			"sds": map[string]any{
				"variant_id": 89764,
			},
		},
	}, func(resp map[string]any) string {
		taskID, _ := resp["task_id"].(string)
		return taskID
	})

	task := waitForTaskResult[listingkit.TaskResult](t, testServer.Client(), testServer.URL+"/api/v1/listing-kits/tasks/"+taskID, listingKitTaskTerminal)
	require.NotEqual(t, listingkit.TaskStatusFailed, task.Status)
	require.NotNil(t, task.Result)
	require.NotNil(t, task.Result.ImageAssets)
	require.NotNil(t, task.Result.SDSSync)
	require.Equal(t, int64(89764), task.Result.SDSSync.VariantID)
	require.Equal(t, "completed", task.Result.SDSSync.Status)
	require.Equal(t, int64(89764), task.Result.SDSSync.ProductID)
	require.Equal(t, int64(14555), task.Result.SDSSync.PrototypeGroupID)
	require.Equal(t, "698744758333792256", task.Result.SDSSync.LayerID)
	require.Equal(t, int64(396548287), task.Result.SDSSync.MaterialID)
	require.Condition(t, func() bool {
		for _, child := range task.Result.ChildTasks {
			if child.Kind == "sds_design_sync" && child.Status == string(listingkit.TaskStatusCompleted) {
				return true
			}
		}
		return false
	})
}

func listingKitTaskTerminal(result listingkit.TaskResult) (bool, string) {
	switch result.Status {
	case listingkit.TaskStatusCompleted, listingkit.TaskStatusNeedsReview, listingkit.TaskStatusFailed:
		return true, string(result.Status)
	default:
		return false, string(result.Status)
	}
}
