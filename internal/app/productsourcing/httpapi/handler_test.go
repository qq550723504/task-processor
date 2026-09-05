package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func TestImportHTTPReportsConflictAndSafeSourceWarnings(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{catalog.ErrPublicationConflict, 409}, {ErrAccessDenied, 403}, {errors.New("private dependency details"), 500}, {ErrInvalidImport, 400},
	} {
		h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
			return ImportResult{SourceWarnings: []sourcing.SourceWarning{{Code: "missing_title", Field: "title", Message: "title unavailable"}}}, test.err
		}))
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"source_account_id":0,"store_id":"s"}`)).WithContext(requestContext()))
		if out.Code != test.status || strings.Contains(out.Body.String(), "private dependency details") {
			t.Fatalf("status/body=%d %s", out.Code, out.Body.String())
		}
		if errors.Is(test.err, ErrInvalidImport) && !strings.Contains(out.Body.String(), "missing_title") {
			t.Fatal("invalid source warnings lost at HTTP boundary")
		}
		if errors.Is(test.err, ErrAccessDenied) && strings.Contains(out.Body.String(), "missing_title") {
			t.Fatal("denied request exposed source facts")
		}
	}
}

func TestImportHTTPBoundsSlowBodyReadsOnRealServer(t *testing.T) {
	called := make(chan struct{}, 1)
	handler := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
		called <- struct{}{}
		return ImportResult{}, nil
	}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(requestContext(), 50*time.Millisecond)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer server.Close()
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Write([]byte("POST /unregistered HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{\"url\":"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 504 {
		t.Fatalf("slow input status %d", response.StatusCode)
	}
	select {
	case <-called:
		t.Fatal("incomplete body reached importer")
	default:
	}
}

type importFunc func(context.Context, ImportCommand) (ImportResult, error)

func (f importFunc) Import(ctx context.Context, c ImportCommand) (ImportResult, error) {
	return f(ctx, c)
}
func requestContext() context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "old-tenant", UserID: "actor", HomeOrganizationID: "home", EffectiveOrganizationID: "selected"})
}

func TestImportHTTPUsesEffectiveIdentityAndBoundedDeadline(t *testing.T) {
	calls := 0
	handler := NewHandler(importFunc(func(ctx context.Context, cmd ImportCommand) (ImportResult, error) {
		calls++
		if cmd.OrganizationID != "selected" || cmd.ActorID != "actor" {
			t.Fatalf("wrong identity: %+v", cmd)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > ImportTimeout {
			t.Fatal("deadline not bounded")
		}
		return ImportResult{}, nil
	}))
	req := httptest.NewRequest(http.MethodPost, "/unregistered", strings.NewReader(`{"url":"https://detail.1688.com/offer/123.html","product":{"id":"123","title":"Bottle"},"source_account_id":0,"store_id":"store-1"}`)).WithContext(requestContext())
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != 200 || calls != 1 {
		t.Fatalf("status %d calls %d: %s", out.Code, calls, out.Body.String())
	}
}

func TestImportHTTPRejectsInvalidInputBeforeImport(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"legacy field", `{"source_store_id":1}`, 400},
		{"tenant spoof", `{"organization_id":"victim"}`, 400},
		{"trailing object", `{} {}`, 400}, {"null", `null`, 400},
		{"empty", ``, 400}, {"negative account", `{"source_account_id":-1}`, 400},
		{"oversize", `{"raw_snapshot":"` + strings.Repeat("x", MaxImportBytes) + `"}`, 413},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) { calls++; return ImportResult{}, nil }))
			out := httptest.NewRecorder()
			h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(test.body)).WithContext(requestContext()))
			if out.Code != test.status || calls != 0 {
				t.Fatalf("status=%d calls=%d", out.Code, calls)
			}
		})
	}
}

func TestImportHTTPAcceptsExactLimitAndRejectsOneMoreByte(t *testing.T) {
	payload := `{"url":"https://detail.1688.com/offer/123.html","product":{"id":"123"},"source_account_id":0,"store_id":"store-1","raw_snapshot":""}`
	for _, extra := range []int{0, 1} {
		body := payload + strings.Repeat(" ", MaxImportBytes-len(payload)+extra)
		if !json.Valid([]byte(body)) {
			t.Fatal("invalid fixture")
		}
		calls := 0
		h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) { calls++; return ImportResult{}, nil }))
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(body)).WithContext(requestContext()))
		if extra == 0 && (out.Code != 200 || calls != 1) || extra == 1 && (out.Code != 413 || calls != 0) {
			t.Fatalf("extra=%d status=%d calls=%d", extra, out.Code, calls)
		}
	}
}

func TestImportHTTPPreservesEarlierDeadlineAndCancellation(t *testing.T) {
	parent, cancel := context.WithTimeout(requestContext(), 20*time.Millisecond)
	defer cancel()
	expected, _ := parent.Deadline()
	h := NewHandler(importFunc(func(ctx context.Context, _ ImportCommand) (ImportResult, error) {
		got, _ := ctx.Deadline()
		if !got.Equal(expected) {
			t.Fatal("parent deadline extended")
		}
		<-ctx.Done()
		return ImportResult{}, ctx.Err()
	}))
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"source_account_id":0,"store_id":"s"}`)).WithContext(parent))
	if out.Code != 504 {
		t.Fatalf("status = %d", out.Code)
	}
	calls := 0
	h = NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) { calls++; return ImportResult{}, nil }))
	for _, ctx := range []context.Context{context.Background(), authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "1", UserID: "actor"})} {
		out = httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{}`)).WithContext(ctx))
		if out.Code != 401 || calls != 0 {
			t.Fatal("legacy/unverified identity accepted")
		}
	}
}
