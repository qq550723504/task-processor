package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	app "task-processor/internal/app/referralregistration"
	"testing"
	"time"
)

type blockingReferralCommands struct {
	referralHTTPSpy
	entered chan struct{}
	release chan struct{}
}

func (b *blockingReferralCommands) Start(context.Context, app.Request) (app.Admission, error) {
	b.entered <- struct{}{}
	<-b.release
	return app.Admission{}, nil
}

func TestReferralHTTPBoundsConcurrentHandlers(t *testing.T) {
	commands := &blockingReferralCommands{entered: make(chan struct{}, 8), release: make(chan struct{})}
	server := buildCurrentApplicationHTTPServer((referralHTTPModule{commands: commands, serviceCredential: strings.Repeat("a", 64)}).routes(), routeAuthDependencies{})
	request := func() *http.Request {
		r := httptest.NewRequest("POST", referralIntentsPath, strings.NewReader(`{"code":"c"}`))
		r.RemoteAddr = "127.0.0.1:1"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Referral-Service-Credential", strings.Repeat("a", 64))
		r.Header.Set("X-Referral-Client-IP", "203.0.113.1")
		r.Header.Set("Idempotency-Key", strings.Repeat("b", 64))
		return r
	}
	var pending sync.WaitGroup
	defer func() { close(commands.release); pending.Wait() }()
	for i := 0; i < 8; i++ {
		pending.Add(1)
		go func() { defer pending.Done(); server.Handler.ServeHTTP(httptest.NewRecorder(), request()) }()
	}
	for i := 0; i < 8; i++ {
		select {
		case <-commands.entered:
		case <-time.After(time.Second):
			t.Fatal("admitted request did not enter")
		}
	}
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, request())
	if w.Code != 429 {
		t.Fatalf("ninth request was not rejected: %d", w.Code)
	}
}

func TestReferralRegistrationRejectsUntrustedCommands(t *testing.T) {
	for _, tc := range []struct{ name, credential, authorization, ip, body string }{
		{name: "missing credential"},
		{name: "wrong credential", credential: "wrong"},
		{name: "user token", authorization: "Bearer user-token"},
		{name: "missing trusted IP", credential: strings.Repeat("a", 64)},
		{name: "identity selection", credential: strings.Repeat("a", 64), ip: "203.0.113.1", body: `{"subject":"other"}`},
		{name: "oversize", credential: strings.Repeat("a", 64), ip: "203.0.113.1", body: strings.Repeat("x", 4097)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := referralHTTPModule{serviceCredential: strings.Repeat("a", 64)}
			server := buildCurrentApplicationHTTPServer(m.routes(), routeAuthDependencies{})
			r := httptest.NewRequest(http.MethodPost, referralIntentsPath, strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:12345"
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Referral-Service-Credential", tc.credential)
			r.Header.Set("X-Referral-Client-IP", tc.ip)
			r.Header.Set("Authorization", tc.authorization)
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, r)
			if w.Code != 400 && w.Code != 401 && w.Code != 413 {
				t.Fatalf("rejection = %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestReferralRegistrationRejectsAmbiguousJSON(t *testing.T) {
	for _, body := range []string{`{"code":"first","code":"second"}`, `{"Code":"c"}`, `{"code":null}`, `{"email":"e","Email":"other"}`} {
		spy := &referralHTTPSpy{}
		server := buildCurrentApplicationHTTPServer((referralHTTPModule{commands: spy, serviceCredential: strings.Repeat("a", 64)}).routes(), routeAuthDependencies{})
		r := httptest.NewRequest("POST", referralIntentsPath, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Referral-Service-Credential", strings.Repeat("a", 64))
		r.Header.Set("X-Referral-Client-IP", "203.0.113.1")
		r.Header.Set("Idempotency-Key", strings.Repeat("b", 64))
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != 400 || len(spy.calls) != 0 {
			t.Errorf("ambiguous input %s reached commands: %d %v", body, w.Code, spy.calls)
		}
	}
}
