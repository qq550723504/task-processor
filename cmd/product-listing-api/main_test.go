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

	"task-processor/internal/app/httpapi"
	"task-processor/internal/productenrich"
)

func TestStart_GenerateProductAndQueryTask(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	shutdownCh := make(chan os.Signal, 1)
	port := 18084
	// config-test.yaml 需要 management.clientSecret 与 openai.apiKey，测试注入环境变量避免编辑全局文件
	oldClientSecret := os.Getenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET")
	oldOpenAIKey := os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY")
	os.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", "test-secret")
	os.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	defer func() {
		if oldClientSecret == "" {
			os.Unsetenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET")
		} else {
			os.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", oldClientSecret)
		}
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

	// 查询任务
	resp, err = http.Get("http://127.0.0.1:" + fmt.Sprint(port) + "/api/v1/products/tasks/" + taskResp.TaskID)
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
