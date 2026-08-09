package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/app/httpapi"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestStart_GenerateProductAndQueryTask(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	shutdownCh := make(chan os.Signal, 1)
	port := 18084
	fixture := newProductListingAPITestFixture(t)
	client := fixture.authenticatedClient()

	options := httpapi.Options{
		ConfigPath:      "../../config/config-test.yaml",
		Port:            port,
		ShutdownSignal:  shutdownCh,
		ShutdownTimeout: time.Second,
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- start(logger, options)
	}()

	// 等待服务就绪
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/health")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				ready = true
				resp.Body.Close()
				break
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		select {
		case err := <-resultCh:
			require.NoError(t, err)
		default:
			t.Fatal("service did not become ready")
		}
	}

	// 创建产品任务
	reqBody := productenrich.GenerateRequest{Text: "测试蓝牙耳机"}
	b, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/products/generate", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var taskResp productenrich.TaskResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&taskResp))
	resp.Body.Close()
	require.NotEmpty(t, taskResp.TaskID)

	// 查询产品任务
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/products/tasks/" + taskResp.TaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 创建图片处理任务
	imgReqBody := productimage.ImageProcessRequest{ImageURLs: []string{fixture.imageURL("photo.png")}, Marketplace: "amazon"}
	b2, err := json.Marshal(imgReqBody)
	require.NoError(t, err)
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/images/process", "application/json", bytes.NewReader(b2))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var imgResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&imgResp))
	resp.Body.Close()
	imgTaskID, ok := imgResp["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, imgTaskID)

	// 查询图片任务
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/images/tasks/" + imgTaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
	waitForProductImageTaskSuccess(t, client, port, imgTaskID)

	// 创建 Amazon listing 任务
	amazonReqBody := amazonlisting.GenerateRequest{
		Marketplace: "amazon",
		Text:        strings.Repeat("durable blue running shoe with breathable mesh upper, cushioned sole, stable fit, and everyday training comfort ", 12),
		ImageURLs: []string{
			fixture.imageURL("amazon-listing.png"),
			fixture.imageURL("amazon-listing-2.png"),
			fixture.imageURL("amazon-listing-3.png"),
		},
	}
	b3, err := json.Marshal(amazonReqBody)
	require.NoError(t, err)
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader(b3))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var amazonResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&amazonResp))
	resp.Body.Close()
	amazonTaskID, ok := amazonResp["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, amazonTaskID)

	// 查询 Amazon task
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/" + amazonTaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
	waitForAmazonListingTaskTerminal(t, client, port, amazonTaskID)
	fixture.assertNoUnhandled()

	// 优雅退出
	shutdownCh <- syscall.SIGTERM

	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("service did not exit after SIGTERM")
	}
}

func TestStart_ErrorPathsAndCleanup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	shutdownCh := make(chan os.Signal, 1)
	port := 18085
	fixture := newProductListingAPITestFixture(t)
	client := fixture.authenticatedClient()

	options := httpapi.Options{
		ConfigPath:     "../../config/config-test.yaml",
		Port:           port,
		ShutdownSignal: shutdownCh,
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- start(logger, options)
	}()

	// 等待服务就绪
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/health")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				ready = true
				resp.Body.Close()
				break
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		select {
		case err := <-resultCh:
			require.NoError(t, err)
		default:
			t.Fatal("service did not become ready")
		}
	}

	// 400 invalid request for product generation
	resp, err := client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/products/generate", "application/json", bytes.NewReader([]byte(`{"text":""}`)))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// 404 when product task not found
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/products/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for image task not found
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/images/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing task not found
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing review nonexistent task
	reviewBody := bytes.NewReader([]byte(`{"action":"approve"}`))
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/nonexistent-id/review", "application/json", reviewBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing submit nonexistent task
	submitBody := bytes.NewReader([]byte(`{"action":"preview"}`))
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/nonexistent-id/submit", "application/json", submitBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing workbench nonexistent task
	resp, err = client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/nonexistent-id/workbench")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 400 for amazon listing generate invalid request
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader([]byte(`{"text":""}`)))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Create an amazon listing task for review/submit branch coverage
	amazonReqBody := amazonlisting.GenerateRequest{
		Marketplace: "amazon",
		Text:        strings.Repeat("durable blue running shoe with breathable mesh upper, cushioned sole, stable fit, and everyday training comfort ", 12),
		ImageURLs: []string{
			fixture.imageURL("amazon-listing.png"),
			fixture.imageURL("amazon-listing-2.png"),
			fixture.imageURL("amazon-listing-3.png"),
		},
	}
	b3, err := json.Marshal(amazonReqBody)
	require.NoError(t, err)
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader(b3))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var amazonResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&amazonResp))
	resp.Body.Close()
	amazonTaskID, ok := amazonResp["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, amazonTaskID)

	// 400 for unsupported review action on existing task
	reviewBody2 := bytes.NewReader([]byte(`{"action":"unsupported"}`))
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/"+amazonTaskID+"/review", "application/json", reviewBody2)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// 400 for invalid submit body on an existing Amazon listing task.
	submitBody2 := bytes.NewReader([]byte(`{"action":`))
	resp, err = client.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/"+amazonTaskID+"/submit", "application/json", submitBody2)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
	waitForAmazonListingTaskTerminal(t, client, port, amazonTaskID)
	fixture.assertNoUnhandled()

	// 优雅退出并确保关闭
	shutdownCh <- syscall.SIGTERM
	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("service did not exit after SIGTERM")
	}
}

const productListingAPITestBearerToken = "product-listing-api-test-token"

type productListingAPITestFixture struct {
	server    *httptest.Server
	t         *testing.T
	mu        sync.Mutex
	unhandled []string
}

func newProductListingAPITestFixture(t *testing.T) *productListingAPITestFixture {
	t.Helper()
	fixture := &productListingAPITestFixture{t: t}
	imageBuffer := &bytes.Buffer{}
	imageData := image.NewRGBA(image.Rect(0, 0, 1, 1))
	imageData.Set(0, 0, color.RGBA{R: 20, G: 40, B: 60, A: 255})
	require.NoError(t, png.Encode(imageBuffer, imageData))
	imageBytes := imageBuffer.Bytes()
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"introspection_endpoint": fixture.server.URL + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			if r.FormValue("token") != productListingAPITestBearerToken {
				http.Error(w, "unexpected bearer token", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "product-listing-api-test-user",
				"user_id":                               "product-listing-api-test-user",
				"urn:zitadel:iam:user:resourceowner:id": "product-listing-api-test-tenant",
				"urn:zitadel:iam:org:project:roles": map[string]any{
					"listingkit_admin": map[string]any{},
				},
			})
		case "/v1/chat/completions":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer sk-test" {
				fixture.recordUnhandled(r.Method + " " + r.URL.Path + " with invalid OpenAI request")
				http.Error(w, "invalid OpenAI request", http.StatusBadRequest)
				return
			}
			content, err := json.Marshal(map[string]any{
				"title":          "Durable Blue Running Shoe",
				"category":       []string{"Shoes", "Running"},
				"attributes":     map[string]string{"color": "blue", "material": "mesh"},
				"selling_points": []string{"Breathable mesh", "Cushioned sole", "Stable fit"},
				"seo_keywords":   []string{"running shoe", "blue trainer"},
				"description":    "Durable blue running shoe with breathable mesh upper, cushioned sole, and stable fit for everyday training comfort.",
				"images": []string{
					fixture.imageURL("amazon-listing.png"),
					fixture.imageURL("amazon-listing-2.png"),
					fixture.imageURL("amazon-listing-3.png"),
				},
			})
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "chatcmpl-product-listing-api-test",
				"object":  "chat.completion",
				"created": 0,
				"model":   "product-listing-api-test-model",
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": string(content),
					},
					"finish_reason": "stop",
				}},
			})
		case "/v1/images/edits", "/v1/images/generations":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer sk-image-test" {
				fixture.recordUnhandled(r.Method + " " + r.URL.Path + " with invalid image request")
				http.Error(w, "invalid image request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 0,
				"data": []map[string]string{{
					"b64_json": base64.StdEncoding.EncodeToString(imageBytes),
				}},
			})
		default:
			if strings.HasPrefix(r.URL.Path, "/images/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				w.Header().Set("Content-Type", "image/png")
				w.Header().Set("Content-Length", fmt.Sprint(len(imageBytes)))
				if r.Method == http.MethodGet {
					_, _ = w.Write(imageBytes)
				}
				return
			}
			fixture.recordUnhandled(r.Method + " " + r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fixture.server.Close)
	t.Cleanup(fixture.assertNoUnhandled)
	t.Setenv("ZITADEL_ISSUER_URL", fixture.server.URL)
	t.Setenv("ZITADEL_CLIENT_ID", "product-listing-api-test-client")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	t.Setenv("TASK_PROCESSOR_OPENAI_BASE_URL", fixture.server.URL+"/v1")
	t.Setenv("TASK_PROCESSOR_OPENAI_MODEL", "product-listing-api-test-model")
	t.Setenv("TASK_PROCESSOR_OPENAI_TIMEOUT", "5")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY", "sk-image-test")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL", fixture.server.URL+"/v1")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE", "openai")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL", "product-listing-api-image-test-model")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_TIMEOUT", "5")

	return fixture
}

func (f *productListingAPITestFixture) authenticatedClient() *http.Client {
	return &http.Client{Transport: productListingAPITestBearerTransport{base: http.DefaultTransport}}
}

func (f *productListingAPITestFixture) imageURL(name string) string {
	return f.server.URL + "/images/" + name
}

func (f *productListingAPITestFixture) recordUnhandled(request string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unhandled = append(f.unhandled, request)
}

func (f *productListingAPITestFixture) assertNoUnhandled() {
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Empty(f.t, f.unhandled, "test dependency fixture received unsupported requests")
}

func waitForProductImageTaskSuccess(t *testing.T, client *http.Client, port int, taskID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/images/tasks/" + taskID)
		require.NoError(t, err)
		var task productimage.TaskResult
		require.NoError(t, json.NewDecoder(response.Body).Decode(&task))
		response.Body.Close()
		switch task.Status {
		case productimage.TaskStatusCompleted, productimage.TaskStatusNeedsReview:
			return
		case productimage.TaskStatusFailed, productimage.TaskStatusRejected:
			t.Fatalf("image task %s ended unsuccessfully: %s", taskID, task.Error)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("image task %s did not reach a terminal successful status", taskID)
}

func waitForAmazonListingTaskTerminal(t *testing.T, client *http.Client, port int, taskID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/" + taskID)
		require.NoError(t, err)
		var task amazonlisting.TaskResult
		require.NoError(t, json.NewDecoder(response.Body).Decode(&task))
		response.Body.Close()
		switch task.Status {
		case amazonlisting.TaskStatusCompleted, amazonlisting.TaskStatusNeedsReview:
			return
		case amazonlisting.TaskStatusFailed, amazonlisting.TaskStatusRejected:
			t.Fatalf("Amazon listing task %s ended unsuccessfully: %s", taskID, task.Error)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("Amazon listing task %s did not reach a terminal status", taskID)
}

type productListingAPITestBearerTransport struct {
	base http.RoundTripper
}

func (t productListingAPITestBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+productListingAPITestBearerToken)
	return t.base.RoundTrip(clone)
}
