package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/app/httpapi"
)

func TestStartServesHealthWithoutLegacyProductRoutes(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	shutdown := make(chan os.Signal, 1)
	port := reserveLocalTCPPort(t)
	result := make(chan error, 1)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	go func() {
		result <- start(logger, httpapi.Options{
			ConfigPath:      "../../config/config-test.yaml",
			Port:            port,
			BindAddress:     "127.0.0.1",
			ShutdownSignal:  shutdown,
			ShutdownTimeout: time.Second,
		})
	}()

	baseURL := "http://127.0.0.1:" + fmt.Sprint(port)
	waitForProductListingAPIReady(t, baseURL, result)
	response, err := http.Get(baseURL + "/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/products/generate"},
		{method: http.MethodGet, path: "/api/v1/products/tasks/task-1"},
		{method: http.MethodPost, path: "/api/v1/images/process"},
		{method: http.MethodGet, path: "/api/v1/images/tasks/task-1"},
		{method: http.MethodPost, path: "/api/v1/images/tasks/task-1/review"},
	} {
		request, requestErr := http.NewRequest(route.method, baseURL+route.path, nil)
		require.NoError(t, requestErr)
		response, requestErr = http.DefaultClient.Do(request)
		require.NoError(t, requestErr)
		require.Equal(t, http.StatusNotFound, response.StatusCode, "%s %s", route.method, route.path)
		response.Body.Close()
	}

	shutdown <- syscall.SIGTERM
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("service did not exit after SIGTERM")
	}
}

func reserveLocalTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func waitForProductListingAPIReady(t *testing.T, baseURL string, result <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-result:
			require.NoError(t, err)
			t.Fatal("service exited before becoming ready")
		default:
		}
		response, err := http.Get(baseURL + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("service did not become ready")
}
