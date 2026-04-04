package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productimage"
)

func TestHTTPE2E_ProductImageAndAmazonListingWorkbench(t *testing.T) {
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

	productModule, err := buildProductModule(logger, deps)
	require.NoError(t, err)
	imageModule, err := buildImageModule(logger, deps)
	require.NoError(t, err)
	amazonModule, err := buildAmazonListingModule(logger, deps)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pools := []worker.WorkerPool{productModule.pool, imageModule.pool, amazonModule.pool}
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

	routerServer := buildHTTPServer(0, productModule.handler, imageModule.handler, amazonModule.handler, nil)
	testServer := httptest.NewServer(routerServer.Handler)
	defer testServer.Close()

	imageServer := newE2EImageServer()
	defer imageServer.Close()
	imageURL := imageServer.URL + "/product.png"
	imageURLs := []string{imageURL, imageURL + "?v=2", imageURL + "?v=3"}

	productTaskID := createTaskViaAPI[productenrich.TaskResponse](t, testServer.Client(), testServer.URL+"/api/v1/products/generate", map[string]any{
		"text":       "高品质蓝牙耳机，支持主动降噪、蓝牙 5.3、30 小时续航、双麦克风通话降噪和轻量化佩戴，适合通勤、会议与运动等多种使用场景。",
		"image_urls": imageURLs,
	}, func(resp productenrich.TaskResponse) string { return resp.TaskID })

	productTask := waitForTaskResult[productenrich.TaskResult](t, testServer.Client(), testServer.URL+"/api/v1/products/tasks/"+productTaskID, productTaskTerminal)
	require.Equal(t, productenrich.TaskStatusCompleted, productTask.Status)
	require.NotNil(t, productTask.ProductJSON)
	require.NotEmpty(t, productTask.ProductJSON.Title)

	imageTaskID := createTaskViaAPI[map[string]any](t, testServer.Client(), testServer.URL+"/api/v1/images/process", map[string]any{
		"marketplace": "amazon",
		"text":        "蓝牙耳机主图",
		"image_urls":  imageURLs,
	}, func(resp map[string]any) string {
		taskID, _ := resp["task_id"].(string)
		return taskID
	})

	imageTask := waitForTaskResult[productimage.TaskResult](t, testServer.Client(), testServer.URL+"/api/v1/images/tasks/"+imageTaskID, imageTaskTerminal)
	require.NotEqual(t, productimage.TaskStatusFailed, imageTask.Status)
	require.NotNil(t, imageTask.Result)
	require.NotNil(t, imageTask.Result.MainImage)
	require.NotNil(t, imageTask.Result.WhiteBgImage)

	amazonTaskID := createTaskViaAPI[map[string]any](t, testServer.Client(), testServer.URL+"/api/v1/amazon/listings/generate", map[string]any{
		"marketplace": "amazon",
		"text":        "高品质蓝牙耳机，支持主动降噪、蓝牙 5.3、30 小时续航和舒适佩戴，适合通勤、会议与运动使用。",
		"image_urls":  imageURLs,
	}, func(resp map[string]any) string {
		taskID, _ := resp["task_id"].(string)
		return taskID
	})

	amazonTask := waitForTaskResult[amazonlisting.TaskResult](t, testServer.Client(), testServer.URL+"/api/v1/amazon/listings/tasks/"+amazonTaskID, amazonTaskTerminal)
	require.NotEqual(t, amazonlisting.TaskStatusFailed, amazonTask.Status)
	require.NotNil(t, amazonTask.Result)
	require.NotNil(t, amazonTask.Result.Export)
	require.NotEmpty(t, amazonTask.Result.Title)

	workbench := getJSON[amazonlisting.TaskWorkbench](t, testServer.Client(), testServer.URL+"/api/v1/amazon/listings/tasks/"+amazonTaskID+"/workbench")
	require.Equal(t, amazonTaskID, workbench.TaskID)
	require.True(t, workbench.Ready)
	require.Len(t, workbench.ChildTasks, 2)
	require.NotNil(t, workbench.ReviewSummary)
	require.GreaterOrEqual(t, workbench.ReviewSummary.TotalCount, 0)
	require.NotEmpty(t, workbench.ActionBuckets)
	var itemWithEvidence *amazonlisting.AmazonReviewItem
	for i := range workbench.ReviewItems {
		if len(workbench.ReviewItems[i].Evidence) > 0 {
			itemWithEvidence = &workbench.ReviewItems[i]
			break
		}
	}
	require.NotNil(t, itemWithEvidence)
	require.NotZero(t, itemWithEvidence.Confidence)
	require.NotEmpty(t, itemWithEvidence.Evidence[0].Type)
	require.NotEmpty(t, itemWithEvidence.Evidence[0].Detail)
}

type e2eWebScraper struct{}

func (e2eWebScraper) Scrape(_ context.Context, _ string) (*productenrich.ScrapedData, error) {
	return &productenrich.ScrapedData{}, nil
}

type e2eLLMManager struct {
	client productenrich.LLMClient
}

func (m *e2eLLMManager) GetClient(_ string) (productenrich.LLMClient, error) {
	return m.client, nil
}

func (m *e2eLLMManager) GetDefaultClient() productenrich.LLMClient {
	return m.client
}

type e2eLLMClient struct{}

func (c *e2eLLMClient) Generate(_ context.Context, prompt string) (string, error) {
	switch {
	case strings.Contains(prompt, "请以 JSON 格式返回评分结果"):
		return `{"score": 86, "reason": "测试评分", "strengths": ["完整"], "weaknesses": []}`, nil
	case strings.Contains(prompt, "Return product JSON with fields") || strings.Contains(prompt, "Generate a complete product JSON"):
		return `{"title":"SoundPeak 蓝牙耳机","category":["Electronics","Headphones"],"attributes":{"brand":"SoundPeak","color":"Black","connectivity":"Bluetooth 5.3","noise_cancellation":"Active"},"selling_points":["主动降噪","30小时续航","佩戴舒适"],"seo_keywords":["蓝牙耳机","降噪耳机","运动耳机"],"description":"SoundPeak 蓝牙耳机支持主动降噪、长续航和舒适佩戴，适合日常通勤与运动使用。"}`, nil
	case strings.Contains(prompt, `"dimensions"`) && strings.Contains(prompt, `"technical"`):
		return `{"dimensions":{"length":12,"width":8,"height":4,"unit":"cm"},"weight":{"value":0.3,"unit":"kg"},"package":{"dimensions":{"length":14,"width":10,"height":6,"unit":"cm"},"weight":{"value":0.4,"unit":"kg"},"quantity":1},"technical":{"material":"plastic","connectivity":"bluetooth 5.3"}}`, nil
	case strings.Contains(prompt, "Return a JSON array:"):
		return `[{"sku":"BT-HEADSET-001","attributes":{"color":"Black"},"price":{"currency":"CNY","amount":129,"compare_at":159,"cost_price":89},"stock":120,"images":[],"is_default":true}]`, nil
	case strings.Contains(prompt, `"product_type"`) && strings.Contains(prompt, `"features"`):
		return `{"product_type":"Bluetooth Headset","attributes":{"color":"Black","connectivity":"Bluetooth 5.3","noise_cancellation":"Active"},"features":["主动降噪","长续航","轻量化佩戴"]}`, nil
	case strings.Contains(prompt, `"title": "a concise product title"`):
		return `{"title":"蓝牙耳机","attributes":{"brand":"SoundPeak","color":"Black","connectivity":"Bluetooth 5.3"},"selling_points":["主动降噪","长续航","轻量化佩戴"]}`, nil
	default:
		return `{"score": 85, "reason": "default", "strengths": [], "weaknesses": []}`, nil
	}
}

func (c *e2eLLMClient) AnalyzeImage(_ context.Context, _ string, prompt string) (string, error) {
	if strings.Contains(prompt, "产品图片质量评估专家") {
		return `{"score": 88, "reason": "测试图片评分", "strengths": ["清晰"], "weaknesses": []}`, nil
	}
	return `{"color":"black","material":"plastic","scene":"studio","usage":"audio"}`, nil
}

func newE2EImageServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeE2EPNG(w)
	}))
}

func writeE2EPNG(w http.ResponseWriter) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 244, B: 248, A: 255})
		}
	}
	for y := 250; y < 950; y++ {
		for x := 250; x < 950; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 60, B: 90, A: 255})
		}
	}
	w.Header().Set("Content-Type", "image/png")
	_ = png.Encode(w, img)
}

func createTaskViaAPI[T any](t *testing.T, client *http.Client, url string, payload any, idFn func(T) string) string {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	taskID := idFn(result)
	require.NotEmpty(t, taskID)
	return taskID
}

func waitForTaskResult[T any](t *testing.T, client *http.Client, url string, terminal func(T) (bool, string)) T {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		result := getJSON[T](t, client, url)
		done, status := terminal(result)
		if done {
			return result
		}
		if status == "failed" {
			t.Fatalf("task at %s failed", url)
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("task at %s did not reach terminal state", url)
	var zero T
	return zero
}

func getJSON[T any](t *testing.T, client *http.Client, url string) T {
	t.Helper()

	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func postJSON[T any](t *testing.T, client *http.Client, url string, payload any) T {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func productTaskTerminal(result productenrich.TaskResult) (bool, string) {
	return result.Status == productenrich.TaskStatusCompleted || result.Status == productenrich.TaskStatusFailed, string(result.Status)
}

func imageTaskTerminal(result productimage.TaskResult) (bool, string) {
	switch result.Status {
	case productimage.TaskStatusCompleted, productimage.TaskStatusNeedsReview, productimage.TaskStatusRejected, productimage.TaskStatusFailed:
		return true, string(result.Status)
	default:
		return false, string(result.Status)
	}
}

func amazonTaskTerminal(result amazonlisting.TaskResult) (bool, string) {
	switch result.Status {
	case amazonlisting.TaskStatusCompleted, amazonlisting.TaskStatusNeedsReview, amazonlisting.TaskStatusRejected, amazonlisting.TaskStatusFailed:
		return true, string(result.Status)
	default:
		return false, string(result.Status)
	}
}
