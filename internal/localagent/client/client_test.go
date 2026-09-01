package client

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	sourcea1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/localagent"
)

func TestNewRejectsNonLoopbackHTTP(t *testing.T) {
	_, err := New("http://api.example.test", "token", nil)
	require.ErrorContains(t, err, "HTTPS")
}

func TestNewRejectsBaseURLWithEmptyQuery(t *testing.T) {
	_, err := New("http://127.0.0.1:18086?", "token", nil)
	require.Error(t, err)
}

func TestClientClaimSendsBearerAndMapsNoJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		require.Equal(t, "/api/v1/local-agent/1688-jobs/claim", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	claim, err := client.Claim(context.Background())
	require.NoError(t, err)
	require.Nil(t, claim)
}

func TestClientClaimJobTargetsCreatedJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/local-agent/1688-jobs/job-1/claim", r.URL.Path)
		_, _ = w.Write([]byte(`{"job_id":"job-1","execution_token":"token","url":"https://detail.1688.com/offer/1052008074197.html","state":"claimed"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	claim, err := client.ClaimJob(context.Background(), "job-1")
	require.NoError(t, err)
	require.Equal(t, "job-1", claim.Job.ID)
}

func TestClientSubmitSuccessUsesSnakeCaseSnapshotPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Contains(t, string(request["product_snapshot"]), `"main_image"`)
		require.Contains(t, string(request["product_snapshot"]), `"min_price"`)
		require.NotContains(t, string(request["product_snapshot"]), `"price_range_count"`)
		require.NotContains(t, string(request["product_snapshot"]), `"MainImage"`)
		_, _ = w.Write([]byte(`{"job_id":"job-1","state":"succeeded"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	snapshot := &sourcea1688.Alibaba1688ProductSnapshot{ID: "1052008074197", URL: "https://detail.1688.com/offer/1052008074197.html", MainImage: "https://img/main.jpg", MinPrice: 12.5}
	if field := reflect.ValueOf(snapshot).Elem().FieldByName("PriceRangeCount"); field.IsValid() && field.CanSet() {
		field.SetInt(2)
	}
	_, err = client.SubmitSuccess(context.Background(), "job-1", "execution", snapshot)
	require.NoError(t, err)
}

func TestNewAppliesDefaultHTTPTimeout(t *testing.T) {
	client, err := New("http://127.0.0.1:18086", "token", nil)
	require.NoError(t, err)
	require.Equal(t, defaultHTTPTimeout, client.HTTPClient.Timeout)
}

func TestClientPreservesOversizedSnapshotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"snapshot_too_large","message":"1688 snapshot exceeds size limit"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	_, err = client.SubmitSuccess(context.Background(), "job-1", "execution", &sourcea1688.Alibaba1688ProductSnapshot{ID: "1052008074197", URL: "https://detail.1688.com/offer/1052008074197.html"})
	require.ErrorIs(t, err, localagent.ErrSnapshotTooLarge)
}

func TestClientPreservesInvalidSnapshotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"snapshot_invalid","message":"1688 product snapshot failed server validation"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	_, err = client.SubmitSuccess(context.Background(), "job-1", "execution", &sourcea1688.Alibaba1688ProductSnapshot{ID: "1052008074197", URL: "https://detail.1688.com/offer/1052008074197.html"})
	require.ErrorIs(t, err, localagent.ErrSnapshotInvalid)
}

func TestClientMapsUnencodableSnapshotToInvalidSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unencodable snapshot must not reach the HTTP server")
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	_, err = client.SubmitSuccess(context.Background(), "job-1", "execution", &sourcea1688.Alibaba1688ProductSnapshot{
		ID: "1052008074197", URL: "https://detail.1688.com/offer/1052008074197.html", MinPrice: math.NaN(),
	})
	require.ErrorIs(t, err, localagent.ErrSnapshotInvalid)
}

func TestClientRejectsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example", http.StatusFound)
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	_, err = client.Claim(context.Background())
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), "elsewhere")
}
