package client

import (
	"context"
	"testing"
)

type stubLocalLoginRefresher struct {
	tenantID int64
	storeID  int64
	calls    int
	err      error
}

func (s *stubLocalLoginRefresher) ForceLogin(_ context.Context, tenantID int64, storeID int64) error {
	s.calls++
	s.tenantID = tenantID
	s.storeID = storeID
	return s.err
}

func TestForceRefreshCookiesUsesLocalLoginRefresher(t *testing.T) {
	stub := &stubLocalLoginRefresher{}
	ConfigureLocalLoginRefresher(stub)
	t.Cleanup(func() { ConfigureLocalLoginRefresher(nil) })
	ConfigureLoginAccount("123", "456")

	manager := NewCookieManager(456, nil)
	if _, err := manager.ForceRefreshCookies(); err == nil {
		t.Fatal("expected cookie load to fail without management client")
	}
	if stub.calls != 1 || stub.tenantID != 123 || stub.storeID != 456 {
		t.Fatalf("unexpected refresher call: %+v", stub)
	}
}
