package openmeter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

func TestClientIngestValidatesBeforeCallingSDK(t *testing.T) {
	// Break caught: removing adapter-side validation would submit an invalid event.
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Ingest(context.Background(), openmeterapi.EventInput{}); err == nil {
		t.Fatal("Ingest() error = nil, want usage-event validation error")
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("Ingest() made %d HTTP requests for an invalid event, want 0", got)
	}
}

func TestClientIngestUsesOfficialV3EventEndpoint(t *testing.T) {
	// Break caught: using a legacy endpoint or non-CloudEvents payload would be rejected.
	event, err := BuildUsageEvent(validUsageFact())
	if err != nil {
		t.Fatalf("BuildUsageEvent() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/openmeter/events" {
			t.Errorf("path = %s, want /openmeter/events", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/cloudevents+json" {
			t.Errorf("Content-Type = %q, want application/cloudevents+json", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode event body: %v", err)
		}
		if body["id"] != event.ID || body["source"] != event.Source || body["subject"] != event.Subject || body["type"] != event.Type {
			t.Errorf("event identity = %#v, want ID=%q source=%q subject=%q type=%q", body, event.ID, event.Source, event.Subject, event.Type)
		}
		data, ok := body["data"].(map[string]any)
		if !ok || data["quantity"] != "1" || data["metric"] != string(MetricStudioDesignJobsSucceeded) {
			t.Errorf("event data = %#v, want count metric with quantity 1", body["data"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Ingest(context.Background(), event); err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
}

func TestClientQueryUsageFiltersBySubject(t *testing.T) {
	// Break caught: omitting the subject filter can aggregate another tenant's usage.
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/openmeter/meters/storage-bytes/query" {
			t.Errorf("path = %s, want /openmeter/meters/storage-bytes/query", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode query body: %v", err)
		}
		filters, ok := body["filters"].(map[string]any)
		if !ok {
			t.Fatalf("filters = %#v, want subject filter", body["filters"])
		}
		dimensions, ok := filters["dimensions"].(map[string]any)
		if !ok {
			t.Fatalf("dimensions = %#v, want subject dimension", filters["dimensions"])
		}
		subject, ok := dimensions["subject"].(map[string]any)
		if !ok || subject["eq"] != "tenant:tenant-123" {
			t.Errorf("subject filter = %#v, want eq tenant:tenant-123", dimensions["subject"])
		}
		if body["from"] != from.Format(time.RFC3339) || body["to"] != to.Format(time.RFC3339) {
			t.Errorf("time range = from %q to %q, want %q to %q", body["from"], body["to"], from.Format(time.RFC3339), to.Format(time.RFC3339))
		}
		_, _ = w.Write([]byte(`{"data":[{"value":"9007199254740993"}]}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	got, err := client.QueryUsage(context.Background(), "storage-bytes", "tenant:tenant-123", from, to)
	if err != nil {
		t.Fatalf("QueryUsage() error = %v", err)
	}
	if got != "9007199254740993" {
		t.Errorf("QueryUsage() = %q, want exact numeric string", got)
	}
}

func TestClientQueryUsageHandlesResultCardinality(t *testing.T) {
	// Break caught: treating multiple tenant rows as one value can silently overcount usage.
	for _, tt := range []struct {
		name     string
		response string
		want     string
		wantErr  bool
	}{
		{name: "empty result", response: `{"data":[]}`, want: "0"},
		{name: "multiple rows", response: `{"data":[{"value":"1"},{"value":"2"}]}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/openmeter/meters/meter-123/query" {
					t.Errorf("request = %s %s, want meter query", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("Authorization = %q, want Bearer test-token", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode query body: %v", err)
				}
				_, _ = w.Write([]byte(tt.response))
			}))
			t.Cleanup(server.Close)

			client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-token", HTTPClient: server.Client()})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			got, err := client.QueryUsage(context.Background(), "meter-123", "tenant:tenant-123", time.Now().UTC(), time.Now().UTC())
			if (err != nil) != tt.wantErr {
				t.Fatalf("QueryUsage() error = %v, want error %t", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("QueryUsage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientListCustomerAccessReturnsFeatureAccess(t *testing.T) {
	// Break caught: using a different entitlement endpoint or dropping feature access results.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/openmeter/customers/customer-123/entitlement-access" {
			t.Errorf("path = %s, want customer entitlement-access path", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"type":"boolean","feature_key":"studio","has_access":true}]}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	access, err := client.ListCustomerAccess(context.Background(), "customer-123")
	if err != nil {
		t.Fatalf("ListCustomerAccess() error = %v", err)
	}
	if len(access) != 1 || access[0].FeatureKey != "studio" || !access[0].HasAccess {
		t.Errorf("ListCustomerAccess() = %#v, want studio access", access)
	}
}
