package safeimagehttp

import (
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
