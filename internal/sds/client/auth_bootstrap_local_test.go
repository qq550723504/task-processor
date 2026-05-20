package client

import (
	"context"
	"path/filepath"
	"testing"

	coreconfig "task-processor/internal/core/config"
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

func TestTriggerLoginServiceLoginUsesLocalProvider(t *testing.T) {
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
	cfg.LoginService = coreconfig.SDSLoginServiceConfig{
		DefaultHeadless: false,
	}
	cfg.AuthFile = filepath.Join(t.TempDir(), "auth.json")
	cfg.CookieFile = filepath.Join(t.TempDir(), "cookies.json")

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	initialCalls := stub.triggerCalls
	if err := c.triggerLoginServiceLogin(context.Background(), true); err != nil {
		t.Fatalf("trigger local login: %v", err)
	}
	if stub.triggerCalls != initialCalls+1 {
		t.Fatalf("expected one additional local browser launch, got calls=%d initial=%d req=%+v", stub.triggerCalls, initialCalls, stub.triggerReq)
	}
	if stub.triggerReq.TenantID != "1" || stub.triggerReq.Identifier != "869" || !stub.triggerReq.ForceLogin || stub.triggerReq.Headless {
		t.Fatalf("unexpected local login request: %+v", stub.triggerReq)
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

func TestHasUsableAuthStateAcceptsMinimalTokenAndMerchant(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuthFile = filepath.Join(t.TempDir(), "auth.json")
	cfg.CookieFile = filepath.Join(t.TempDir(), "cookies.json")

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	c.authState = &AuthState{
		AccessToken: "token",
		MerchantID:  36811,
		UserID:      30098709,
	}
	c.cookies = nil

	if !c.hasUsableAuthState() {
		t.Fatal("expected auth state with token and merchant/user to be usable without cookies")
	}
}
