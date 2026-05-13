package client

import (
	"context"
	"path/filepath"
	"testing"
)

type stubLocalSDSLoginProvider struct {
	triggerReq   LocalLoginRequest
	triggerCalls int
	payload      *LocalAuthPayload
}

func (s *stubLocalSDSLoginProvider) TriggerLogin(_ context.Context, req LocalLoginRequest) error {
	s.triggerCalls++
	s.triggerReq = req
	return nil
}

func (s *stubLocalSDSLoginProvider) LoadAuthState(context.Context, string, string) (*LocalAuthPayload, error) {
	return s.payload, nil
}

func TestTriggerLoginServiceLoginDoesNotAutoLaunchLocalProvider(t *testing.T) {
	stub := &stubLocalSDSLoginProvider{}
	ConfigureLocalLoginProvider(stub)
	t.Cleanup(func() { ConfigureLocalLoginProvider(nil) })

	cfg := DefaultConfig()
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginServiceTenantID:   "1",
		LoginServiceIdentifier: "869",
		LoginMerchantName:      "merchant",
		LoginUsername:          "user",
		LoginPassword:          "secret",
	}
	cfg.AuthFile = filepath.Join(t.TempDir(), "auth.json")
	cfg.CookieFile = filepath.Join(t.TempDir(), "cookies.json")

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.triggerLoginServiceLogin(context.Background(), true); err != nil {
		t.Fatalf("trigger local login: %v", err)
	}
	if stub.triggerCalls != 0 {
		t.Fatalf("expected no local browser launch, got calls=%d req=%+v", stub.triggerCalls, stub.triggerReq)
	}
}

func TestLoadLoginServiceBootstrapUsesLocalProvider(t *testing.T) {
	stub := &stubLocalSDSLoginProvider{
		payload: &LocalAuthPayload{
			AccessToken: "token",
			OutToken:    "out",
			MerchantID:  12,
			UserID:      34,
			Username:    "demo",
			Source:      "local-login",
			Cookies: []*PersistedCookie{
				{Name: "sid", Value: "ok", Domain: ".sdsdiy.com", Path: "/"},
			},
		},
	}
	ConfigureLocalLoginProvider(stub)
	t.Cleanup(func() { ConfigureLocalLoginProvider(nil) })

	cfg := DefaultConfig()
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginServiceTenantID:   "1",
		LoginServiceIdentifier: "869",
	}
	cfg.AuthFile = filepath.Join(t.TempDir(), "auth.json")
	cfg.CookieFile = filepath.Join(t.TempDir(), "cookies.json")

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	material, err := c.loadLoginServiceBootstrap(context.Background())
	if err != nil {
		t.Fatalf("load bootstrap: %v", err)
	}
	if material == nil || material.authState == nil || material.authState.AccessToken != "token" || len(material.cookies) != 1 || material.source != "local-login" {
		t.Fatalf("unexpected bootstrap material: %+v", material)
	}
}
