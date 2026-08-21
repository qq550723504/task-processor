package local

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type localRoundTripFunc func(*http.Request) (*http.Response, error)

func (f localRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewImageDownloaderUsesStrictPublicClient(t *testing.T) {
	downloader := NewImageDownloader(3 * time.Second)
	if downloader == nil || downloader.client == nil {
		t.Fatal("NewImageDownloader() returned an unconfigured downloader")
	}
	if downloader.client.Timeout != 3*time.Second {
		t.Fatalf("client timeout = %s, want 3s", downloader.client.Timeout)
	}
	transport, ok := downloader.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport = %T, want *http.Transport", downloader.client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("image downloader retains an environment proxy")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("image downloader allows insecure TLS verification")
	}
}

func TestImageDownloaderRejectsNonHTTPSBeforeTransport(t *testing.T) {
	called := false
	downloader := &ImageDownloader{client: &http.Client{Transport: localRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("unexpected")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}

	if _, err := downloader.DownloadImage("http://example.com/image"); err == nil {
		t.Fatal("DownloadImage() accepted a non-HTTPS URL")
	}
	if called {
		t.Fatal("DownloadImage() invoked the transport for a rejected URL")
	}
}

func TestImageDownloaderDownloadsBoundedBody(t *testing.T) {
	downloader := &ImageDownloader{client: &http.Client{Transport: localRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("image-bytes")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}

	data, err := downloader.DownloadImage("https://example.com/image")
	if err != nil {
		t.Fatalf("DownloadImage() error = %v", err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("DownloadImage() data = %q, want image-bytes", data)
	}
}
