package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/imroc/req/v3"
)

func TestBuildClientDefaultsToStrictTLS(t *testing.T) {
	client := Build(ClientConfig{})

	tlsConfig := client.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config to be set")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("expected strict tls verification by default")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %v, want %v", tlsConfig.MinVersion, tls.VersionTLS12)
	}
	if tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("MaxVersion = %v, want %v", tlsConfig.MaxVersion, tls.VersionTLS13)
	}
}

func TestBuildClientAppliesRequestSettings(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test"); got != "value" {
			t.Fatalf("header X-Test = %q, want %q", got, "value")
		}
		if atomic.AddInt32(&attempts, 1) < 3 {
			http.Error(w, "retry", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Build(ClientConfig{
		Timeout:            45 * time.Second,
		RetryCount:         7,
		InsecureSkipVerify: true,
		Headers: map[string]string{
			"X-Test": "value",
		},
		RetryCondition: func(resp *req.Response, err error) bool {
			return err == nil && resp != nil && resp.StatusCode >= 500
		},
	})

	tlsConfig := client.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config to be set")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Fatal("expected explicit insecure tls flag to be applied")
	}

	if got := client.GetClient().Timeout; got != 45*time.Second {
		t.Fatalf("timeout = %v, want %v", got, 45*time.Second)
	}
	resp, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempt count = %d, want %d", got, 3)
	}
}
