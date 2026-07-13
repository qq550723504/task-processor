package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// config-test.yaml needs openai.apiKey; inject it for tests instead of editing the shared file.
	oldOpenAIKey := os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY")
	os.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	defer func() {
		if oldOpenAIKey == "" {
			os.Unsetenv("TASK_PROCESSOR_OPENAI_API_KEY")
		} else {
			os.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", oldOpenAIKey)
		}
	}()

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

	resp, err := http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/products/generate", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var taskResp productenrich.TaskResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&taskResp))
	resp.Body.Close()
	require.NotEmpty(t, taskResp.TaskID)

	// 查询产品任务
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/products/tasks/" + taskResp.TaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 创建图片处理任务
	imgReqBody := productimage.ImageProcessRequest{ImageURLs: []string{"https://example.com/photo.jpg"}, Marketplace: "amazon"}
	b2, err := json.Marshal(imgReqBody)
	require.NoError(t, err)
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/images/process", "application/json", bytes.NewReader(b2))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var imgResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&imgResp))
	resp.Body.Close()
	imgTaskID, ok := imgResp["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, imgTaskID)

	// 查询图片任务
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/images/tasks/" + imgTaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 创建 Amazon listing 任务
	amazonReqBody := amazonlisting.GenerateRequest{
		Marketplace: "amazon",
		Text:        "高品质运动鞋",
		ImageURLs:   []string{"https://example.com/amazon-listing.jpg"},
	}
	b3, err := json.Marshal(amazonReqBody)
	require.NoError(t, err)
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader(b3))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var amazonResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&amazonResp))
	resp.Body.Close()
	amazonTaskID, ok := amazonResp["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, amazonTaskID)

	// 查询 Amazon task
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/" + amazonTaskID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

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
	oldOpenAIKey := os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY")
	os.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	defer func() {
		if oldOpenAIKey == "" {
			os.Unsetenv("TASK_PROCESSOR_OPENAI_API_KEY")
		} else {
			os.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", oldOpenAIKey)
		}
	}()

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
	resp, err := http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/products/generate", "application/json", bytes.NewReader([]byte(`{"text":""}`)))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// 404 when product task not found
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/products/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for image task not found
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/images/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing task not found
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/nonexistent-id")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing review nonexistent task
	reviewBody := bytes.NewReader([]byte(`{"action":"approve"}`))
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/nonexistent-id/review", "application/json", reviewBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing submit nonexistent task
	submitBody := bytes.NewReader([]byte(`{"action":"preview"}`))
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/nonexistent-id/submit", "application/json", submitBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 404 for amazon listing workbench nonexistent task
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/amazon/listings/tasks/nonexistent-id/workbench")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 400 for amazon listing generate invalid request
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader([]byte(`{"text":""}`)))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Create an amazon listing task for review/submit branch coverage
	amazonReqBody := amazonlisting.GenerateRequest{
		Marketplace: "amazon",
		Text:        "检测内容",
		ImageURLs:   []string{"https://example.com/amazon-review.jpg"},
	}
	b3, err := json.Marshal(amazonReqBody)
	require.NoError(t, err)
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/generate", "application/json", bytes.NewReader(b3))
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
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/"+amazonTaskID+"/review", "application/json", reviewBody2)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// 400 for submit if task result is empty (invalid state for submission)
	submitBody2 := bytes.NewReader([]byte(`{"action":"preview"}`))
	resp, err = http.Post("http://127.0.0.1:"+fmt.Sprint(port)+"/api/v1/amazon/listings/tasks/"+amazonTaskID+"/submit", "application/json", submitBody2)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// 优雅退出并确保关闭
	shutdownCh <- syscall.SIGTERM
	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("service did not exit after SIGTERM")
	}
}
