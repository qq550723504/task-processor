package safeimagehttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewPublicImageHTTPClientDisablesProxy(t *testing.T) {
	client := NewPublicImageHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("public image transport retains an environment proxy")
	}
}

func TestNewPublicImageHTTPClientStopsLongRedirectChains(t *testing.T) {
	client := NewPublicImageHTTPClient()
	request, err := http.NewRequest(http.MethodGet, "https://example.com/image", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	via := make([]*http.Request, 10)
	if err := client.CheckRedirect(request, via); err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("CheckRedirect() error = %v, want ten-redirect limit error", err)
	}
}

func TestResolvePublicImageHostIPsHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolvePublicImageHostIPs(ctx, "example.com")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolvePublicImageHostIPs() error = %v, want context canceled", err)
	}
}

func TestDownloadRejectsDeclaredBodyOverLimit(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: 11,
			Body:          io.NopCloser(strings.NewReader("body must not be read")),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}

	data, err := Download(context.Background(), client, "https://example.com/image", 10)
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("Download() error = %v, want body limit error", err)
	}
	if data != nil {
		t.Fatalf("Download() data = %d bytes, want nil", len(data))
	}
	if !called {
		t.Fatal("Download() did not call the injected transport for a valid URL")
	}
}

func TestDownloadRejectsStreamedBodyOverLimit(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: -1,
			Body:          io.NopCloser(strings.NewReader("01234567890")),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}

	data, err := Download(context.Background(), client, "https://example.com/image", 10)
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("Download() error = %v, want body limit error", err)
	}
	if data != nil {
		t.Fatalf("Download() data = %d bytes, want nil", len(data))
	}
}
