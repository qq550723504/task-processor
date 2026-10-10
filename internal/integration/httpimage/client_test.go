package httpimage

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestIsPrivateIPRejectsIPv6SiteLocalAndSpecialUseRanges(t *testing.T) {
	for _, raw := range []string{"fec0::1", "2001:db8::1"} {
		ip := net.ParseIP(raw)
		if !IsPrivateIP(ip) {
			t.Fatalf("IsPrivateIP(%q) = false, want special-use IPv6 address rejected", raw)
		}
	}
}

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

func TestNewPublicImageHTTPClientRejectsPrivateRedirectTargets(t *testing.T) {
	client := NewPublicImageHTTPClient()
	request, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/image", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	if err := client.CheckRedirect(request, nil); err == nil || !strings.Contains(err.Error(), "public https url is required") {
		t.Fatalf("CheckRedirect() error = %v, want private target rejected", err)
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
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Download() error = %v, want typed definite body-limit failure", err)
	}
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
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Download() error = %v, want typed definite body-limit failure", err)
	}
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("Download() error = %v, want body limit error", err)
	}
	if data != nil {
		t.Fatalf("Download() data = %d bytes, want nil", len(data))
	}
}

type rejectedResponseBody struct{ read, closed bool }

func (b *rejectedResponseBody) Read([]byte) (int, error) { b.read = true; return 0, io.EOF }
func (b *rejectedResponseBody) Close() error             { b.closed = true; return nil }

func TestDownloadPreservesRejectedResponseStatus(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusGone, http.StatusNotFound, http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &rejectedResponseBody{}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method %s", req.Method)
				}
				return &http.Response{StatusCode: status, Body: body, Header: make(http.Header), Request: req}, nil
			})}
			data, err := Download(context.Background(), client, "https://example.com/image?signature=private", 10)
			var response *HTTPStatusError
			if !errors.As(err, &response) || response.StatusCode != status {
				t.Fatalf("Download() error = %v, want status %d", err, status)
			}
			if data != nil || body.read || !body.closed {
				t.Fatal("rejected response must close its unread body without returning bytes")
			}
			if strings.Contains(err.Error(), "signature") {
				t.Fatal("download error exposed signed result locator")
			}
		})
	}
}
