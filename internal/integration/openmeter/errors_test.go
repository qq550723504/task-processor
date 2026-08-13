package openmeter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"syscall"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

func TestClassifyErrorUsesOfficialAPIError(t *testing.T) {
	// Break caught: treating an authentication error as retryable would create a futile retry loop.
	for _, tt := range []struct {
		name string
		code int
		want FailureKind
	}{
		{name: "request timeout", code: http.StatusRequestTimeout, want: FailureRetryable},
		{name: "rate limited", code: http.StatusTooManyRequests, want: FailureRetryable},
		{name: "server failure", code: http.StatusBadGateway, want: FailureRetryable},
		{name: "bad request", code: http.StatusBadRequest, want: FailurePermanent},
		{name: "not found", code: http.StatusNotFound, want: FailurePermanent},
		{name: "conflict", code: http.StatusConflict, want: FailurePermanent},
		{name: "unprocessable", code: http.StatusUnprocessableEntity, want: FailurePermanent},
		{name: "unauthorized", code: http.StatusUnauthorized, want: FailureConfiguration},
		{name: "forbidden", code: http.StatusForbidden, want: FailureConfiguration},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/openmeter/meters/meter-123/query" {
					t.Errorf("request = %s %s, want meter query", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "" {
					t.Errorf("Authorization = %q, want empty without API key", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode query body: %v", err)
				}
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(tt.code)
				_, _ = fmt.Fprintf(w, `{"status":%d,"title":"Rejected","detail":"safe detail"}`, tt.code)
			}))
			t.Cleanup(server.Close)

			client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			_, err = client.QueryUsage(context.Background(), "meter-123", "tenant:tenant-123", time.Now().UTC(), time.Now().UTC())
			if err == nil {
				t.Fatal("QueryUsage() error = nil, want API error")
			}
			apiErr, ok := openmeterapi.AsAPIError(err)
			if !ok {
				t.Fatalf("AsAPIError(%v) = false, want official API error", err)
			}
			if apiErr.StatusCode != tt.code {
				t.Errorf("APIError.StatusCode = %d, want %d", apiErr.StatusCode, tt.code)
			}
			if apiErr.Title != "Rejected" || apiErr.Detail != "safe detail" {
				t.Errorf("APIError detail = title %q detail %q, want safe response detail", apiErr.Title, apiErr.Detail)
			}
			if got := ClassifyError(err); got != tt.want {
				t.Errorf("ClassifyError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyErrorHandlesNetworkAndConfigurationFailures(t *testing.T) {
	// Break caught: transient transport failures or invalid client configuration would be misrouted.
	timeout := &net.DNSError{IsTimeout: true}
	temporary := temporaryNetworkError{}
	for _, tt := range []struct {
		name string
		err  error
		want FailureKind
	}{
		{name: "connection reset", err: &url.Error{Err: syscall.ECONNRESET}, want: FailureRetryable},
		{name: "wrapped connection refused", err: fmt.Errorf("ingest request failed: %w", &url.Error{Op: "Post", URL: "http://127.0.0.1:48888/api/v3/openmeter/events", Err: syscall.ECONNREFUSED}), want: FailureRetryable},
		{name: "timeout", err: timeout, want: FailureRetryable},
		{name: "temporary network", err: temporary, want: FailureRetryable},
		{name: "unknown", err: errors.New("unexpected response shape"), want: FailurePermanent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != tt.want {
				t.Errorf("ClassifyError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}

	for _, baseURL := range []string{"", "://bad-url"} {
		t.Run("invalid base URL "+baseURL, func(t *testing.T) {
			_, err := NewClient(Config{BaseURL: baseURL})
			if err == nil {
				t.Fatal("NewClient() error = nil, want configuration error")
			}
			if got := ClassifyError(err); got != FailureConfiguration {
				t.Errorf("ClassifyError(%v) = %q, want %q", err, got, FailureConfiguration)
			}
		})
	}
}

func TestClassifyErrorRetriesPlatformConnectionRefusal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close loopback listener: %v", err)
	}

	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatal("dial closed loopback listener error = nil, want connection refusal")
	}
	if got := ClassifyError(fmt.Errorf("ingest request failed: %w", err)); got != FailureRetryable {
		t.Fatalf("ClassifyError(platform connection refusal %v) = %q, want %q", err, got, FailureRetryable)
	}
}

type temporaryNetworkError struct{}

func (temporaryNetworkError) Error() string   { return "temporary network failure" }
func (temporaryNetworkError) Timeout() bool   { return false }
func (temporaryNetworkError) Temporary() bool { return true }
