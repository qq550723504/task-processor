package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	app "task-processor/internal/app/referralregistration"
)

type panicReferralCommands struct {
	referralHTTPSpy
	value any
}

func (p panicReferralCommands) Start(context.Context, app.Request) (app.Admission, error) {
	panic(p.value)
}
func (p panicReferralCommands) Resume(context.Context, string, string) error {
	panic(p.value)
}

func TestReferralPanicDoesNotExposePrivateValues(t *testing.T) {
	var logs bytes.Buffer
	previous := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	defer func() { gin.DefaultErrorWriter = previous }()
	previousMode := gin.Mode()
	defer gin.SetMode(previousMode)
	for _, mode := range []string{gin.DebugMode, gin.ReleaseMode} {
		gin.SetMode(mode)
		for _, value := range []any{"synthetic-private-provider-value", &net.OpError{Op: "write", Err: fmt.Errorf("synthetic-private-provider-value: %w", syscall.EPIPE)}} {
			for _, path := range []string{referralIntentsPath, referralResumePath} {
				logs.Reset()
				server := buildCurrentApplicationHTTPServer((referralHTTPModule{commands: &panicReferralCommands{value: value}, serviceCredential: strings.Repeat("a", 64)}).routes(), routeAuthDependencies{})
				r := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
				r.RemoteAddr = "127.0.0.1:1"
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-Referral-Service-Credential", strings.Repeat("a", 64))
				r.Header.Set("X-Referral-Client-IP", "203.0.113.1")
				r.Header.Set("Idempotency-Key", strings.Repeat("b", 64))
				w := httptest.NewRecorder()
				server.Handler.ServeHTTP(w, r)
				if strings.Contains(logs.String(), strings.Repeat("a", 64)) || strings.Contains(logs.String(), "synthetic-private-provider-value") {
					t.Errorf("%s panic leaked synthetic private values (log bytes=%d)", path, logs.Len())
				}
				if w.Code != 503 || strings.Contains(w.Body.String(), "synthetic-private") {
					t.Errorf("%s panic did not return a safe unknown outcome", path)
				}
			}
		}
	}
}

func TestReferralSlowSocketBodyReleasesCapacityWithinRequestBudget(t *testing.T) {
	for _, path := range []string{referralIntentsPath, referralResumePath} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			server := buildCurrentApplicationHTTPServer((referralHTTPModule{serviceCredential: strings.Repeat("a", 64)}).routes(), routeAuthDependencies{})
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			defer func() { _ = server.Close(); <-done }()
			started := time.Now()
			var sockets []net.Conn
			defer func() {
				for _, c := range sockets {
					_ = c.Close()
				}
			}()
			for i := 0; i < 8; i++ {
				c, e := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
				if e != nil {
					t.Fatal(e)
				}
				sockets = append(sockets, c)
				_, e = fmt.Fprintf(c, "POST %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 128\r\nX-Referral-Service-Credential: %s\r\nX-Referral-Client-IP: 203.0.113.1\r\n\r\n{", path, strings.Repeat("a", 64))
				if e != nil {
					t.Fatal(e)
				}
			}
			client := &http.Client{Timeout: time.Second}
			defer client.CloseIdleConnections()
			probe := func() int {
				r, _ := http.NewRequest("POST", "http://"+listener.Addr().String()+referralIntentsPath, strings.NewReader(`{}`))
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-Referral-Service-Credential", strings.Repeat("a", 64))
				r.Header.Set("X-Referral-Client-IP", "203.0.113.1")
				res, e := client.Do(r)
				if e != nil {
					t.Fatal(e)
				}
				defer res.Body.Close()
				_, _ = io.Copy(io.Discard, res.Body)
				return res.StatusCode
			}
			for deadline := time.Now().Add(time.Second); probe() != 429; {
				if time.Now().After(deadline) {
					t.Fatal("eight slow bodies did not occupy capacity")
				}
				time.Sleep(10 * time.Millisecond)
			}
			for _, c := range sockets {
				_ = c.SetReadDeadline(started.Add(17 * time.Second))
				res, e := http.ReadResponse(bufio.NewReader(c), nil)
				if e != nil {
					t.Errorf("slow body still blocked after %.2fs: %v", time.Since(started).Seconds(), e)
					break
				}
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
				if res.StatusCode != 400 && res.StatusCode != 408 {
					t.Errorf("slow body status %d", res.StatusCode)
				}
			}
			if status := probe(); status == 429 {
				t.Error("expired slow bodies retained all eight slots")
			}
			t.Logf("real HTTP/1.1 %s socket/capacity probe elapsed %.3fs (server ReadTimeout=%s)", path, time.Since(started).Seconds(), server.ReadTimeout)
		})
	}
}

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
