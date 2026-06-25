package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/shein/api"

	"github.com/imroc/req/v3"
)

func TestBaseAPIClientRefreshesAuthOnBusinessAuthExpired(t *testing.T) {
	t.Parallel()

	var refreshCalls int
	var requestCalls int
	fresh := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCalls++
		cookie, err := r.Cookie("session")
		if !fresh || err != nil || cookie.Value != "fresh" {
			_ = jsonEncode(w, api.APIResponse{
				Code: "20302",
				Msg:  "子系统登录重定向",
			})
			return
		}
		_ = jsonEncode(w, api.APIResponse{
			Code: "0",
			Msg:  "ok",
		})
	}))
	defer server.Close()

	httpClient := req.C()
	httpClient.SetCommonCookies(&http.Cookie{Name: "session", Value: "stale", Path: "/"})
	baseClient := NewBaseAPIClient(server.URL, 227, 869, httpClient)
	baseClient.SetAuthRefreshFunc(func() error {
		refreshCalls++
		fresh = true
		httpClient.ClearCookies()
		httpClient.SetCommonCookies(&http.Cookie{Name: "session", Value: "fresh", Path: "/"})
		return nil
	})

	var result api.APIResponse
	if err := baseClient.APIRequest(http.MethodPost, server.URL, map[string]any{"ok": true}, &result); err != nil {
		t.Fatalf("APIRequest() error = %v", err)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}
	if requestCalls != 2 {
		t.Fatalf("requestCalls = %d, want 2", requestCalls)
	}
	if result.Code != "0" {
		t.Fatalf("result.Code = %q, want 0", result.Code)
	}
}

func TestBaseAPIClientDoesNotRefreshAuthOnCostPriceForbidden(t *testing.T) {
	t.Parallel()

	var refreshCalls int
	var requestCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCalls++
		http.Error(w, "cost price forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	baseClient := NewBaseAPIClient(server.URL, 227, 870, req.C())
	baseClient.SetAuthRefreshFunc(func() error {
		refreshCalls++
		return nil
	})

	var result api.APIResponse
	err := baseClient.APIRequest(http.MethodPost, server.URL+GetQueryCostPriceEndpoint(), map[string]any{"ok": true}, &result)
	if err == nil {
		t.Fatal("APIRequest() error = nil, want forbidden error")
	}
	if refreshCalls != 0 {
		t.Fatalf("refreshCalls = %d, want 0", refreshCalls)
	}
	if requestCalls != 1 {
		t.Fatalf("requestCalls = %d, want 1", requestCalls)
	}
}

func jsonEncode(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(value)
}
