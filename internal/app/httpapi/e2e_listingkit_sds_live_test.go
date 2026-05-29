package httpapi

import (
	"context"
	"net/http/httptest"
	"os"
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
	sdsclient "task-processor/internal/sds/client"
	sdsusecase "task-processor/internal/sds/usecase"
)

func TestHTTPLiveE2E_ListingKitGenerateSyncsSDSDesign(t *testing.T) {
	if os.Getenv("SDS_LIVE_E2E") != "1" {
		t.Skip("set SDS_LIVE_E2E=1 to enable live SDS e2e")
	}

	repoRoot, err := filepath.Abs("../../../")
	require.NoError(t, err)
	liveConfig := sdsclient.DefaultConfig()
	liveConfig.AuthFile = filepath.Join(repoRoot, "data", "sds", "auth_state.json")
	liveConfig.CookieFile = filepath.Join(repoRoot, "data", "sds", "session_cookies.json")
	authClient, err := sdsclient.New(liveConfig)
	require.NoError(t, err)
	authState := authClient.AuthState()
	if authState == nil || authState.AccessToken == "" {
		t.Skip("data/sds/auth_state.json not found or accessToken is empty")
	}

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
		sdsHTTPClient, err := sdsclient.New(liveConfig)
		if err != nil {
			return nil, nil, err
		}
		auth := sdsHTTPClient.AuthState()
		if auth == nil || auth.AccessToken == "" {
			return nil, auth, nil
		}
		svc, err := sdsusecase.NewService(sdsusecase.Config{
			SDSClient:    sdsHTTPClient,
			ImageService: imageSvc,
		})
		if err != nil {
			return nil, auth, err
		}
		return svc, auth, nil
	}

	productModule, err := buildProductModule(logger, deps)
	require.NoError(t, err)
	imageModule, err := buildImageModule(logger, deps)
	require.NoError(t, err)
	listingKitModule, err := buildListingKitModule(logger, deps)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pools := []worker.WorkerPool{productModule.Pool, imageModule.Pool, listingKitModule.Pool}
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

	routerServer := buildHTTPServer(0, productModule.Handler, imageModule.Handler, nil, listingKitModule.Handler, nil)
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
	if task.Result.SDSSync.Status != "completed" {
		t.Fatalf("sds sync failed: %+v, warnings=%+v", task.Result.SDSSync, task.Result.Summary)
	}
	require.Equal(t, "completed", task.Result.SDSSync.Status)
	require.Greater(t, task.Result.SDSSync.MaterialID, int64(0))
	require.NotEmpty(t, task.Result.SDSSync.LayerID)
	require.Greater(t, task.Result.SDSSync.PrototypeGroupID, int64(0))
	require.Condition(t, func() bool {
		for _, child := range task.Result.ChildTasks {
			if child.Kind == "sds_design_sync" && child.Status == string(listingkit.TaskStatusCompleted) {
				return true
			}
		}
		return false
	})
}
