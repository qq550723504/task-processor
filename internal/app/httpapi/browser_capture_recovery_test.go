package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"task-processor/internal/httproute"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
)

// Only synthetic markers enter this task-owned buffer. Never print the dump on failure.
const recoveryTestKey = "10000000-0000-4000-8000-000000000399"
const recoveryPanicMarker = "ISSUE399_SYNTHETIC_PANIC_PRIVATE"

type browserRecoveryPanicService struct{ failure any }

func (s browserRecoveryPanicService) Capture(context.Context, string, []byte) (sourcing.AcquisitionResult, error) {
	panic(s.failure)
}
func (s browserRecoveryPanicService) Verify(context.Context, string, []byte) (sourcing.AcquisitionResult, error) {
	panic(s.failure)
}
func (s browserRecoveryPanicService) ByKey(context.Context, string) (sourcing.AcquisitionResult, error) {
	panic(s.failure)
}
func (s browserRecoveryPanicService) Read(context.Context, string) (sourcing.AcquisitionResult, error) {
	panic(s.failure)
}

func TestBrowserCaptureMountedRecoveryDoesNotLogRequestSecrets(t *testing.T) {
	oldMode, oldWriter, oldDebugWriter := gin.Mode(), gin.DefaultErrorWriter, gin.DefaultWriter
	t.Cleanup(func() { gin.SetMode(oldMode); gin.DefaultErrorWriter = oldWriter; gin.DefaultWriter = oldDebugWriter })
	for _, mode := range []string{gin.DebugMode, gin.ReleaseMode} {
		for _, kind := range []string{"panic", "broken-pipe", "connection-reset"} {
			for _, endpoint := range []struct{ method, suffix string }{
				{http.MethodPost, ""}, {http.MethodPost, "/verify"},
				{http.MethodGet, "/by-key/" + recoveryTestKey}, {http.MethodGet, "/" + recoveryTestKey},
			} {
				t.Run(mode+"/"+kind+"/"+endpoint.method+endpoint.suffix, func(t *testing.T) {
					gin.SetMode(mode)
					var output bytes.Buffer
					gin.DefaultErrorWriter = &output
					gin.DefaultWriter = &output
					var failure any = recoveryPanicMarker
					if kind != "panic" {
						code := syscall.EPIPE
						if kind == "connection-reset" {
							code = syscall.ECONNRESET
						}
						failure = &net.OpError{Op: "write", Net: "tcp", Err: &os.SyscallError{Syscall: "write", Err: code}}
					}
					deps := newRouteAuthDependencies()
					deps.workbenchVerifier = titleVerifier{}
					deps.organizationResolver = workbenchcontext.NewResolver(&titleGrants{}, "project", "v1", nil)
					// Fault injection isolates recovery, not permission or persistence acceptance.
					deps.roleMiddleware = func(httproute.Descriptor) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }
					routes := browserCaptureRoutes(browserRecoveryPanicService{failure}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil })
					var observed *gin.Context
					for i := range routes {
						handler := routes[i].Handler
						routes[i].Handler = func(c *gin.Context) { observed = c; handler(c) }
					}
					server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, deps)
					body := ""
					if endpoint.method == http.MethodPost {
						body = `{"captureVersion":1,"evidence":{"schemaVersion":1,"sourceURL":"https://detail.1688.com/offer/981645030344.html","offerID":"981645030344","title":"ISSUE399_SYNTHETIC_BODY_PRIVATE","description":null,"attributes":[],"variants":[],"priceFacts":[],"images":[],"capturedAt":"2026-09-12T00:00:00Z","contentSHA256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","parserVersion":"1688-browser-dom/v1","warnings":[],"missingFacts":[]}}`
					}
					request := httptest.NewRequest(endpoint.method, browserCaptureBase+endpoint.suffix, strings.NewReader(body))
					request.Header.Set("Authorization", "Bearer operator")
					request.Header.Set("X-Requested-Organization-ID", "B")
					request.Header.Set("Idempotency-Key", recoveryTestKey)
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Cookie", "fixture=ISSUE399_SYNTHETIC_COOKIE_PRIVATE")
					request.Header.Set("X-Request-ID", "ISSUE399_SYNTHETIC_REQUEST_PRIVATE")
					response := httptest.NewRecorder()
					server.Handler.ServeHTTP(response, request)
					if kind == "panic" && response.Code != http.StatusInternalServerError {
						t.Fatalf("panic status=%d, want 500", response.Code)
					}
					if observed == nil || !observed.IsAborted() {
						t.Fatal("recovery did not abort the mounted handler")
					}
					if kind != "panic" && (response.Code == http.StatusInternalServerError || response.Body.Len() != 0) {
						t.Error("broken connection recovery attempted a response")
					}
					logged := output.String()
					for _, marker := range []string{recoveryTestKey, recoveryPanicMarker, "ISSUE399_SYNTHETIC_BODY_PRIVATE", "ISSUE399_SYNTHETIC_COOKIE_PRIVATE", "ISSUE399_SYNTHETIC_REQUEST_PRIVATE", "Bearer operator"} {
						if strings.Contains(logged, marker) {
							t.Error("recovery diagnostic contains a forbidden synthetic request/panic marker")
						}
					}
					if !strings.Contains(logged, "event=browser_capture_recovery method=") {
						t.Error("missing safe recovery event")
					}
					if !strings.Contains(logged, "browserRecoveryPanicService") {
						t.Error("missing safe source-level recovery callsite")
					}
				})
			}
		}
	}
}

type browserRecoveryTestSink func([]byte) (int, error)

func (f browserRecoveryTestSink) Write(p []byte) (int, error) { return f(p) }

func TestBrowserCaptureRecoveryWriterFragmentsFailuresAndConcurrency(t *testing.T) {
	var output bytes.Buffer
	w := browserCaptureRecoveryWriter{sink: &browserCaptureRecoverySink{writer: &output}, method: http.MethodGet, route: browserCaptureBase + "/by-key/:key"}
	for _, part := range []string{"SECRET", "_FRAGMENT", recoveryTestKey, recoveryPanicMarker} {
		n, err := w.Write([]byte(part))
		if err != nil || n != len(part) {
			t.Fatal("safe writer did not consume input fragment")
		}
	}
	for _, forbidden := range []string{"SECRET", "_FRAGMENT", recoveryTestKey, recoveryPanicMarker} {
		if strings.Contains(output.String(), forbidden) {
			t.Error("fragment leaked into safe diagnostic")
		}
	}
	if strings.Count(output.String(), "event=browser_capture_recovery ") != 4 {
		t.Error("missing fragment replacement diagnostics")
	}
	for _, partial := range []bool{false, true} {
		calls := 0
		w.sink = &browserCaptureRecoverySink{writer: browserRecoveryTestSink(func(p []byte) (int, error) {
			calls++
			if bytes.Contains(p, []byte(recoveryTestKey)) {
				t.Error("sink received raw request key")
			}
			if partial {
				return 1, nil
			}
			return 0, errors.New("synthetic sink unavailable")
		})}
		n, err := w.Write([]byte(recoveryTestKey))
		if n != len(recoveryTestKey) || err == nil || calls != 1 {
			t.Error("sink failure must consume input without fallback/retry")
		}
		if partial && !errors.Is(err, io.ErrShortWrite) {
			t.Error("partial sink write not reported")
		}
	}
	output.Reset()
	w.sink = &browserCaptureRecoverySink{writer: &output}
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 8 {
				_, _ = w.Write([]byte(recoveryTestKey))
			}
		}()
	}
	workers.Wait()
	if strings.Count(output.String(), "event=browser_capture_recovery ") != 256 {
		t.Error("concurrent diagnostics were lost or interleaved")
	}
	if strings.Contains(output.String(), recoveryTestKey) {
		t.Error("concurrent request key leaked")
	}
}

func TestBrowserCaptureRecoveryExactRoutingLeavesOtherRecoveryUnchanged(t *testing.T) {
	oldMode, oldWriter := gin.Mode(), gin.DefaultErrorWriter
	t.Cleanup(func() { gin.SetMode(oldMode); gin.DefaultErrorWriter = oldWriter })
	gin.SetMode(gin.ReleaseMode)
	for _, candidate := range []struct{ method, template, target string }{
		{http.MethodGet, "/outside/:key", "/outside/synthetic"},
		{http.MethodPost, browserCaptureBase + "/by-key/:key", browserCaptureBase + "/by-key/synthetic"},
		{http.MethodGet, browserCaptureBase + "/extra/path", browserCaptureBase + "/extra/path"},
		{http.MethodGet, "", "/not-mounted"},
	} {
		var output bytes.Buffer
		gin.DefaultErrorWriter = &output
		router := gin.New()
		router.Use(browserCaptureRecovery())
		fail := func(*gin.Context) { panic("SYNTHETIC_UNCHANGED_RECOVERY") }
		if candidate.template == "" {
			router.NoRoute(fail)
		} else {
			router.Handle(candidate.method, candidate.template, fail)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(candidate.method, candidate.target, nil))
		if response.Code != 500 || !strings.Contains(output.String(), "SYNTHETIC_UNCHANGED_RECOVERY") || strings.Contains(output.String(), "event=browser_capture_recovery ") {
			t.Error("non-allowlisted recovery changed")
		}
	}
}
