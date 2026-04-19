package browser

import (
	"errors"
	"testing"
)

func TestErrorDetectorGetErrorType(t *testing.T) {
	detector := NewErrorDetector()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "product not found", err: errors.New("页面不存在(404)"), want: "product_not_found"},
		{name: "auth", err: errors.New("SIGN_IN_REQUIRED: login required"), want: "authentication"},
		{name: "captcha", err: errors.New("captcha challenge detected"), want: "captcha"},
		{name: "browser crash", err: errors.New("Page crashed while rendering"), want: "browser_crash"},
		{name: "server", err: errors.New("503 service unavailable"), want: "server_error"},
		{name: "network", err: errors.New("network error: connection reset"), want: "network"},
		{name: "timeout", err: errors.New("navigation timeout exceeded"), want: "timeout"},
		{name: "unknown", err: errors.New("some other error"), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.GetErrorType(tt.err); got != tt.want {
				t.Fatalf("GetErrorType(%q)=%q want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestErrorDetectorIsBlockedOrSeriousError(t *testing.T) {
	detector := NewErrorDetector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "timeout is serious", err: errors.New("navigation timeout exceeded"), want: true},
		{name: "forbidden is serious", err: errors.New("access denied by upstream"), want: true},
		{name: "websocket crash is serious", err: errors.New("Socket connection to remote was closed"), want: true},
		{name: "404 is not serious", err: errors.New("页面不存在(404)"), want: false},
		{name: "nil is not serious", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.IsBlockedOrSeriousError(tt.err); got != tt.want {
				t.Fatalf("IsBlockedOrSeriousError(%v)=%v want %v", tt.err, got, tt.want)
			}
		})
	}
}
