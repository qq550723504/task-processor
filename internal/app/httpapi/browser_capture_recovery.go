package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/gin-gonic/gin"
)

// Keep Gin's recovery/abort semantics. Only these four matched routes replace
// request/panic dumps with bounded source-level diagnostics.
func browserCaptureRecovery() gin.HandlerFunc {
	ordinary := gin.Recovery()
	sink := &browserCaptureRecoverySink{writer: gin.DefaultErrorWriter}
	return func(c *gin.Context) {
		method, route, ok := browserCaptureRecoveryLabels(c.Request.Method, c.FullPath())
		if !ok {
			ordinary(c)
			return
		}
		gin.RecoveryWithWriter(browserCaptureRecoveryWriter{sink: sink, method: method, route: route})(c)
	}
}

func browserCaptureRecoveryLabels(method, route string) (string, string, bool) {
	switch {
	case method == http.MethodPost && route == browserCaptureBase:
		return http.MethodPost, browserCaptureBase, true
	case method == http.MethodPost && route == browserCaptureBase+"/verify":
		return http.MethodPost, browserCaptureBase + "/verify", true
	case method == http.MethodGet && route == browserCaptureBase+"/by-key/:key":
		return http.MethodGet, browserCaptureBase + "/by-key/:key", true
	case method == http.MethodGet && route == browserCaptureBase+"/:operation_id":
		return http.MethodGet, browserCaptureBase + "/:operation_id", true
	default:
		return "", "", false
	}
}

type browserCaptureRecoverySink struct {
	mu     sync.Mutex
	writer io.Writer
}

type browserCaptureRecoveryWriter struct {
	sink          *browserCaptureRecoverySink
	method, route string
}

func (w browserCaptureRecoveryWriter) Write(p []byte) (int, error) {
	// Never inspect, retain, split, or format any original log bytes. Every
	// fragment is independently replaced, including broken-pipe diagnostics.
	var record bytes.Buffer
	fmt.Fprintf(&record, "event=browser_capture_recovery method=%s route=%s\n", w.method, w.route)
	var pcs [48]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if frame.Function != "" {
			fmt.Fprintf(&record, "  %s:%d\n", frame.Function, frame.Line)
		}
		if !more {
			break
		}
	}
	if w.sink.writer == nil {
		return len(p), nil
	}
	w.sink.mu.Lock()
	n, err := w.sink.writer.Write(record.Bytes())
	w.sink.mu.Unlock()
	if err == nil && n != record.Len() {
		err = io.ErrShortWrite
	}
	// Consume all input even on sink failure. Never retry/fallback with raw data.
	return len(p), err
}
