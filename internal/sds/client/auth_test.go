package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSetAuthState(t *testing.T) {
	t.Parallel()

	c, err := New(nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	c.SetAuthState(&AuthState{
		AccessToken: "token-1",
		OutToken:    "token-2",
		MerchantID:  42,
	})

	state := c.AuthState()
	if state == nil {
		t.Fatal("expected auth state")
	}
	if state.AccessToken != "token-1" || state.OutToken != "token-2" {
		t.Fatalf("unexpected auth state: %+v", state)
	}
}

func TestLoginReturnsCaptchaRequiredWithoutPersistingEmptyAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
			"msg": "SUCCESS",
			"data": map[string]any{
				"verifyCaptcha": map[string]any{
					"code":      "Success",
					"message":   "success",
					"requestId": "AE680023-7EE2-5661-B044-FB5F851681C0",
					"result": map[string]any{
						"verifyCode":   "F003",
						"verifyResult": false,
					},
					"success": true,
				},
			},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = c.Login(context.Background(), LoginRequest{
		MerchantName:       "xuweixia",
		Username:           "tester",
		Password:           "secret",
		DomainName:         "www.sdsdiy.com",
		VerifyCaptchaParam: "verify-payload",
		ExtraInfo:          `{"fingerprintId":"abc"}`,
	})
	if err == nil {
		t.Fatal("expected login to require captcha")
	}

	var captchaErr *CaptchaRequiredError
	if !errors.As(err, &captchaErr) {
		t.Fatalf("expected CaptchaRequiredError, got %T: %v", err, err)
	}
	if captchaErr.VerifyCode != "F003" || captchaErr.VerifyState {
		t.Fatalf("unexpected captcha error: %+v", captchaErr)
	}
	if state := c.AuthState(); state != nil {
		t.Fatalf("expected auth state to remain nil, got %+v", state)
	}
	persistedState, loadErr := NewAuthStateStore(cfg.AuthFile).Load()
	if loadErr != nil {
		t.Fatalf("load auth file: %v", loadErr)
	}
	if persistedState != nil {
		t.Fatalf("expected no persisted auth state, got %+v", persistedState)
	}
}
