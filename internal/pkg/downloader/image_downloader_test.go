package downloader

import (
	"testing"

	"task-processor/internal/pkg/httpclient"
)

func TestNewImageDownloaderDefaultsToStrictTLS(t *testing.T) {
	downloader := NewImageDownloader()

	tlsConfig := downloader.httpClient.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("expected strict tls verification by default")
	}
}

func TestNewImageDownloaderWithConfigAllowsExplicitTLSBypass(t *testing.T) {
	downloader := NewImageDownloaderWithConfig(httpclient.ClientConfig{
		InsecureSkipVerify: true,
	})

	tlsConfig := downloader.httpClient.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify to be enabled")
	}
}
