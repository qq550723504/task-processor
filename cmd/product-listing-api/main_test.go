package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
)

func TestStartFailsClosedWithoutPersistentConsumerStores(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	err := start(logger, defaultTestOptions())
	if err == nil || !strings.Contains(err.Error(), "listingkit database config is required") {
		t.Fatalf("start() error = %v, want persistent ListingKit repository failure", err)
	}
}

func TestStartLifecycleServesHealthRejectsLegacyRoutesAndStopsOnSIGTERM(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	shutdown := make(chan os.Signal, 1)
	port := reserveLocalTCPPort(t)
	options := httpapi.Options{
		Port:            port,
		BindAddress:     "127.0.0.1",
		ShutdownSignal:  shutdown,
		ShutdownTimeout: time.Second,
	}
	done := make(chan error, 1)
	go func() {
		done <- startWithApplication(logger, options, runCommandLifecycleSmokeApplication)
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForCommandSmokeReady(t, baseURL, done)
	response, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", response.StatusCode)
	}
	for _, path := range []string{"/api/v1/products/tasks/task-1", "/api/v1/images/tasks/task-1"} {
		response, err = http.Get(baseURL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, response.StatusCode)
		}
	}

	shutdown <- syscall.SIGTERM
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("startWithApplication() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("command lifecycle did not stop after SIGTERM")
	}
}

func runCommandLifecycleSmokeApplication(_ *logrus.Logger, serviceName string, options httpapi.Options) error {
	if serviceName != "product listing API service" {
		return fmt.Errorf("service name = %q", serviceName)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	server := &http.Server{
		Addr:    net.JoinHostPort(options.BindAddress, fmt.Sprint(options.Port)),
		Handler: mux,
	}
	serverErr := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		serverErr <- err
	}()
	select {
	case err := <-serverErr:
		return err
	case <-options.ShutdownSignal:
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(ctx)
}

func reserveLocalTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved port: %v", err)
	}
	return port
}

func waitForCommandSmokeReady(t *testing.T, baseURL string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("command lifecycle exited before readiness: %v", err)
		default:
		}
		response, err := http.Get(baseURL + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("command lifecycle did not become ready")
}

func defaultTestOptions() httpapi.Options {
	return httpapi.Options{
		ConfigPath: "../../config/config-test.yaml",
		Port:       0,
	}
}
