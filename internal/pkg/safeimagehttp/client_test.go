package safeimagehttp

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

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

func TestResolvePublicImageHostIPsHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolvePublicImageHostIPs(ctx, "example.com")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolvePublicImageHostIPs() error = %v, want context canceled", err)
	}
}
