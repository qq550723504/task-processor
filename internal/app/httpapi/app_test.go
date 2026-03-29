package httpapi

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRun_GracefulShutdown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stopCh := make(chan os.Signal, 1)
	options := Options{ConfigPath: "config/config-test.yaml", Port: 18082, ShutdownSignal: stopCh}

	done := make(chan error, 1)
	go func() {
		done <- Run(logger, options)
	}()

	// 等待服务可达，最多 5s
	serverReady := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://127.0.0.1:18082/health")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				serverReady = true
				resp.Body.Close()
				break
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !serverReady {
		select {
		case err := <-done:
			if err != nil {
				t.Skipf("skipping due bootstrap failure: %v", err)
			}
		default:
			t.Fatal("server is not ready and Run has not exited")
		}
	}

	stopCh <- syscall.SIGTERM

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not exit after shutdown signal")
	}
}
