package userdirectory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestClientListByTenantPaginatesStableSearchUsersQuery(t *testing.T) {
	t.Parallel()

	const (
		tenantID = "tenant-101"
		token    = "directory-token"
	)

	type searchRequest struct {
		Query struct {
			Offset string `json:"offset"`
			Limit  uint64 `json:"limit"`
			Asc    bool   `json:"asc"`
		} `json:"query"`
		SortingColumn string `json:"sortingColumn"`
		Queries       []struct {
			OrganizationIDQuery struct {
				OrganizationID string `json:"organizationId"`
			} `json:"organizationIdQuery"`
		} `json:"queries"`
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPost || r.URL.Path != "/v2/users" {
			t.Errorf("request = %s %s, want POST /v2/users", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization = %q, want bearer directory token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var request searchRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if request.Query.Limit != 100 || !request.Query.Asc {
			t.Errorf("query = %#v, want limit 100 and ascending order", request.Query)
		}
		if request.SortingColumn != "USER_FIELD_NAME_CREATION_DATE" {
			t.Errorf("sortingColumn = %q", request.SortingColumn)
		}
		if len(request.Queries) != 1 || request.Queries[0].OrganizationIDQuery.OrganizationID != tenantID {
			t.Errorf("queries = %#v, want tenant organization query", request.Queries)
		}

		w.Header().Set("Content-Type", "application/json")
		switch request.Query.Offset {
		case "0":
			_, _ = w.Write([]byte(`{
				"details":{"totalResult":"3"},
				"result":[
					{"userId":"subject-a","state":"USER_STATE_ACTIVE","username":"private-login","details":{"resourceOwner":"tenant-101"},"human":{"profile":{"displayName":"Private Name"},"email":{"email":"private@example.test"}}},
					{"userId":"subject-b","details":{"resourceOwner":"tenant-101"}}
				]
			}`))
		case "2":
			_, _ = w.Write([]byte(`{
				"details":{"totalResult":3},
				"result":[{"userId":"subject-c","details":{"resourceOwner":"tenant-101"}}]
			}`))
		default:
			t.Errorf("offset = %q, want 0 then 2", request.Query.Offset)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	directory, err := NewClient(ClientConfig{
		IssuerURL: "  " + server.URL + "///  ",
		Token:     "  " + token + "  ",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	users, err := directory.ListByTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	want := []User{
		{Subject: "subject-a", TenantID: tenantID},
		{Subject: "subject-b", TenantID: tenantID},
		{Subject: "subject-c", TenantID: tenantID},
	}
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("users = %#v, want %#v", users, want)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
}

func TestClientListByTenantRejectsCrossTenantRowsWithoutLeakingIdentifiers(t *testing.T) {
	t.Parallel()

	const (
		requestedTenant = "tenant-requested-secret"
		returnedTenant  = "tenant-returned-secret"
		returnedSubject = "subject-returned-secret"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"details":{"totalResult":"1"},"result":[{"userId":"` + returnedSubject + `","details":{"resourceOwner":"` + returnedTenant + `"}}]}`))
	}))
	defer server.Close()

	directory, err := NewClient(ClientConfig{IssuerURL: server.URL, Token: "directory-token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = directory.ListByTenant(context.Background(), requestedTenant)
	if err == nil {
		t.Fatal("ListByTenant error = nil, want cross-tenant response rejection")
	}
	assertOmitsSecrets(t, err.Error(), requestedTenant, returnedTenant, returnedSubject)
}

func TestClientListByTenantSanitizesNonSuccessResponse(t *testing.T) {
	t.Parallel()

	const (
		tenantID = "tenant-secret"
		token    = "token-secret"
		body     = "private@example.test Private Name mocked-response-body"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, body+" "+tenantID+" "+token, http.StatusForbidden)
	}))
	defer server.Close()

	directory, err := NewClient(ClientConfig{IssuerURL: server.URL, Token: token, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = directory.ListByTenant(context.Background(), tenantID)
	if err == nil {
		t.Fatal("ListByTenant error = nil, want non-2xx error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %q, want status", err)
	}
	assertOmitsSecrets(t, err.Error(), tenantID, token, body, "private@example.test", "Private Name", "mocked-response-body")
}

func TestClientListByTenantSanitizesTransportFailure(t *testing.T) {
	t.Parallel()

	const (
		tenantID = "tenant-secret"
		token    = "token-secret"
	)
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed " + tenantID + " " + token + " private@example.test Private Name mocked-response-body")
	})}
	directory, err := NewClient(ClientConfig{IssuerURL: "https://directory.invalid", Token: token, HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = directory.ListByTenant(context.Background(), tenantID)
	if err == nil {
		t.Fatal("ListByTenant error = nil, want transport error")
	}
	if !strings.Contains(err.Error(), "list ZITADEL users") {
		t.Fatalf("error = %q, want operation context", err)
	}
	assertOmitsSecrets(t, err.Error(), tenantID, token, "private@example.test", "Private Name", "mocked-response-body")
}

func TestClientListByTenantPreservesContextCancellation(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	directory, err := NewClient(ClientConfig{IssuerURL: "https://directory.invalid", Token: "directory-token", HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = directory.ListByTenant(ctx, "tenant-secret")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListByTenant error = %v, want context canceled", err)
	}
}

func TestNewClientRequiresIssuerAndReadOnlyToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  ClientConfig
	}{
		{name: "issuer", cfg: ClientConfig{Token: "token"}},
		{name: "token", cfg: ClientConfig{IssuerURL: "https://issuer.example"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(test.cfg); err == nil {
				t.Fatal("NewClient error = nil, want configuration error")
			}
		})
	}
}

func TestNewClientUsesBoundedDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	directory, err := NewClient(ClientConfig{IssuerURL: "https://issuer.example", Token: "directory-token"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	actual, ok := directory.(*client)
	if !ok {
		t.Fatalf("NewClient returned %T, want *client", directory)
	}
	if actual.http == http.DefaultClient {
		t.Fatal("default HTTP client = http.DefaultClient, want dedicated bounded client")
	}
	if actual.http.Timeout <= 0 {
		t.Fatalf("default HTTP client timeout = %s, want bounded timeout", actual.http.Timeout)
	}
}

func TestNewClientRetainsConfiguredHTTPClient(t *testing.T) {
	t.Parallel()

	expected := &http.Client{}
	directory, err := NewClient(ClientConfig{
		IssuerURL:  "https://issuer.example",
		Token:      "directory-token",
		HTTPClient: expected,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	actual, ok := directory.(*client)
	if !ok {
		t.Fatalf("NewClient returned %T, want *client", directory)
	}
	if actual.http != expected {
		t.Fatalf("HTTP client = %p, want configured client %p", actual.http, expected)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func assertOmitsSecrets(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(output, secret) {
			t.Errorf("output contains secret %q: %q", secret, output)
		}
	}
}
