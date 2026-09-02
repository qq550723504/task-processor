package zitadel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

const authorizationListPath = "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations"

type capturedAuthorizationListRequest struct {
	Pagination struct {
		Offset int  `json:"offset"`
		Limit  int  `json:"limit"`
		Asc    bool `json:"asc"`
	} `json:"pagination"`
	SortingColumn string                       `json:"sortingColumn"`
	Filters       []map[string]json.RawMessage `json:"filters"`
}

func TestAuthorizationClientUsesOfficialV2ContractAndSyntheticFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/list_authorizations_two_orgs.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, authorizationListPath, r.URL.Path)
		assert.Equal(t, "Bearer synthetic-user-token", r.Header.Get("Authorization"))
		assert.Equal(t, "1", r.Header.Get("Connect-Protocol-Version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, 0, request.Pagination.Offset)
		assert.Equal(t, 100, request.Pagination.Limit)
		assert.True(t, request.Pagination.Asc)
		assert.Equal(t, "AUTHORIZATION_FIELD_NAME_ID", request.SortingColumn)
		require.Len(t, request.Filters, 2)
		assert.JSONEq(t, `{"ids":["synthetic-user-000001"]}`, string(request.Filters[0]["inUserIds"]))
		assert.JSONEq(t, `{"id":"synthetic-project-000001"}`, string(request.Filters[1]["projectId"]))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(fixture)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL+"/", server.Client())
	got, err := client.ListOwnProjectAuthorizations(
		context.Background(),
		" synthetic-user-token ",
		" synthetic-user-000001 ",
		" synthetic-project-000001 ",
	)

	require.NoError(t, err)
	assert.Equal(t, []authidentity.OrganizationGrant{
		{
			OrganizationID:   "synthetic-acceptance-org-a-000001",
			OrganizationName: "ListingKit Acceptance Organization A",
			ProjectID:        "synthetic-project-000001",
			Roles:            []string{"listingkit_admin"},
		},
		{
			OrganizationID:   "synthetic-acceptance-org-b-000001",
			OrganizationName: "ListingKit Acceptance Organization B",
			ProjectID:        "synthetic-project-000001",
			Roles:            []string{"listingkit_viewer"},
		},
	}, got)
}

func TestAuthorizationClientPaginatesWithStableOffsets(t *testing.T) {
	var (
		mu      sync.Mutex
		offsets []int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		mu.Lock()
		offsets = append(offsets, request.Pagination.Offset)
		mu.Unlock()

		assert.Equal(t, 100, request.Pagination.Limit)
		assert.True(t, request.Pagination.Asc)
		assert.Equal(t, "AUTHORIZATION_FIELD_NAME_ID", request.SortingColumn)

		switch request.Pagination.Offset {
		case 0:
			writeAuthorizationListResponse(t, w, 2, []map[string]any{
				authorizationFixture("auth-1", "user-1", "project-1", "org-b", "Organization B", "STATE_ACTIVE", "viewer"),
			})
		case 1:
			writeAuthorizationListResponse(t, w, 2, []map[string]any{
				authorizationFixture("auth-2", "user-1", "project-1", "org-a", "Organization A", "STATE_ACTIVE", "admin"),
			})
		default:
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	got, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.NoError(t, err)
	assert.Equal(t, []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", OrganizationName: "Organization A", ProjectID: "project-1", Roles: []string{"admin"}},
		{OrganizationID: "org-b", OrganizationName: "Organization B", ProjectID: "project-1", Roles: []string{"viewer"}},
	}, got)
	mu.Lock()
	assert.Equal(t, []int{0, 1}, offsets)
	mu.Unlock()
}

func TestAuthorizationClientAcceptsProtoJSONUint64Pagination(t *testing.T) {
	testCases := []struct {
		name        string
		totalResult any
	}{
		{name: "canonical decimal string", totalResult: "1"},
		{name: "numeric compatibility", totalResult: 1},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeAuthorizationListResponseValue(t, w, tt.totalResult, []map[string]any{
					authorizationFixture("opaque-auth-id", "user-1", "project-1", "org-1", "Organization 1", "STATE_ACTIVE", "viewer"),
				})
			}))
			defer server.Close()

			client := NewAuthorizationClient(server.URL, server.Client())
			got, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

			require.NoError(t, err)
			assert.Equal(t, []authidentity.OrganizationGrant{{
				OrganizationID: "org-1", OrganizationName: "Organization 1", ProjectID: "project-1", Roles: []string{"viewer"},
			}}, got)
		})
	}
}

func TestAuthorizationClientRequiresCompleteSuccessfulResponse(t *testing.T) {
	testCases := []struct {
		name          string
		responseBody  string
		expectedError string
	}{
		{name: "missing pagination", responseBody: `{}`, expectedError: "missing pagination"},
		{name: "missing total result", responseBody: `{"pagination":{},"authorizations":[]}`, expectedError: "missing totalResult"},
		{name: "missing authorizations", responseBody: `{"pagination":{"totalResult":"0"}}`, expectedError: "missing authorizations"},
		{name: "null pagination", responseBody: `{"pagination":null,"authorizations":[]}`, expectedError: "missing pagination"},
		{name: "null total result", responseBody: `{"pagination":{"totalResult":null},"authorizations":[]}`, expectedError: "missing totalResult"},
		{name: "null authorizations", responseBody: `{"pagination":{"totalResult":"0"},"authorizations":null}`, expectedError: "missing authorizations"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tt.responseBody))
				require.NoError(t, err)
			}))
			defer server.Close()

			client := NewAuthorizationClient(server.URL, server.Client())
			_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestAuthorizationClientAcceptsExplicitEmptySuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAuthorizationListResponseValue(t, w, "0", []map[string]any{})
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	got, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestAuthorizationClientRejectsRepeatedAuthorizationIDAcrossNonEmptyPages(t *testing.T) {
	const opaqueAuthorizationID = "opaque/auth:id-not-an-integer"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		switch request.Pagination.Offset {
		case 0:
			writeAuthorizationListResponseValue(t, w, 2, []map[string]any{
				authorizationFixture(opaqueAuthorizationID, "user-1", "project-1", "org-a", "Organization A", "STATE_ACTIVE", "admin"),
			})
		case 1:
			writeAuthorizationListResponseValue(t, w, 2, []map[string]any{
				authorizationFixture(opaqueAuthorizationID, "user-1", "project-1", "org-b", "Organization B", "STATE_ACTIVE", "viewer"),
			})
		default:
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate authorization id")
}

func TestAuthorizationClientRejectsChangedTotalResultAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		switch request.Pagination.Offset {
		case 0:
			writeAuthorizationListResponseValue(t, w, 2, []map[string]any{
				authorizationFixture("auth-1", "user-1", "project-1", "org-a", "Organization A", "STATE_ACTIVE", "admin"),
			})
		case 1:
			writeAuthorizationListResponseValue(t, w, 3, []map[string]any{
				authorizationFixture("auth-2", "user-1", "project-1", "org-b", "Organization B", "STATE_ACTIVE", "viewer"),
			})
		case 2:
			writeAuthorizationListResponseValue(t, w, 3, []map[string]any{
				authorizationFixture("auth-3", "user-1", "project-1", "org-c", "Organization C", "STATE_ACTIVE", "operator"),
			})
		default:
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "total result changed")
}

func TestAuthorizationClientRejectsBlankAuthorizationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAuthorizationListResponseValue(t, w, 1, []map[string]any{
			authorizationFixture(" ", "user-1", "project-1", "org-1", "Organization 1", "STATE_ACTIVE", "viewer"),
		})
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blank authorization id")
}

func TestAuthorizationClientFiltersUntrustedItemsAndDeduplicatesScopedRoles(t *testing.T) {
	authorizations := []map[string]any{
		authorizationFixture("auth-valid-1", "user-1", "project-1", "org-a", "Organization A", "STATE_ACTIVE", "viewer", "admin", "viewer", " "),
		authorizationFixture("auth-valid-2", "user-1", "project-1", "org-a", "Organization A", "STATE_ACTIVE", "operator", "admin"),
		authorizationFixture("auth-wrong-user", "user-2", "project-1", "org-user-2", "Wrong User", "STATE_ACTIVE", "admin"),
		authorizationFixture("auth-wrong-project", "user-1", "project-2", "org-project-2", "Wrong Project", "STATE_ACTIVE", "admin"),
		authorizationFixture("auth-inactive", "user-1", "project-1", "org-inactive", "Inactive Organization", "STATE_INACTIVE", "admin"),
	}
	server := newAuthorizationListServer(t, len(authorizations), authorizations)
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	got, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.NoError(t, err)
	assert.Equal(t, []authidentity.OrganizationGrant{{
		OrganizationID: "org-a", OrganizationName: "Organization A", ProjectID: "project-1",
		Roles: []string{"admin", "operator", "viewer"},
	}}, got)
}

func TestAuthorizationClientRejectsConflictingOrganizationNamesAcrossRawPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		switch request.Pagination.Offset {
		case 0:
			writeAuthorizationListResponse(t, w, 2, []map[string]any{
				authorizationFixture("auth-1", "user-1", "project-1", "org-a", " Organization Alpha ", "STATE_ACTIVE", "viewer"),
			})
		case 1:
			writeAuthorizationListResponse(t, w, 2, []map[string]any{
				authorizationFixture("auth-2", "user-1", "project-1", "org-a", "Organization Beta", "STATE_ACTIVE", "admin"),
			})
		default:
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Equal(t, "ZITADEL authorization contains conflicting organization names", err.Error())
	assert.NotContains(t, err.Error(), "Alpha")
	assert.NotContains(t, err.Error(), "Beta")
}

func TestAuthorizationClientRejectsBlankIdentityBoundaryIDs(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "organization id",
			mutate: func(authorization map[string]any) {
				authorization["organization"].(map[string]any)["id"] = " "
			},
		},
		{
			name: "project id",
			mutate: func(authorization map[string]any) {
				authorization["project"].(map[string]any)["id"] = " "
			},
		},
		{
			name: "user id",
			mutate: func(authorization map[string]any) {
				authorization["user"].(map[string]any)["id"] = " "
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			authorization := authorizationFixture("auth-1", "user-1", "project-1", "org-1", "Organization 1", "STATE_ACTIVE", "viewer")
			tt.mutate(authorization)
			server := newAuthorizationListServer(t, 1, []map[string]any{authorization})
			defer server.Close()

			client := NewAuthorizationClient(server.URL, server.Client())
			_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "blank "+tt.name)
		})
	}
}

func TestAuthorizationClientRejectsDependencyFailuresWithoutSensitiveResponseData(t *testing.T) {
	const bearerToken = "secret-user-bearer-token"
	testCases := []struct {
		name          string
		status        int
		responseBody  string
		expectedError string
	}{
		{name: "non-2xx", status: http.StatusServiceUnavailable, responseBody: `{"message":"secret-response-body"}`, expectedError: "503"},
		{name: "malformed JSON", status: http.StatusOK, responseBody: `secret-malformed-response-body{`, expectedError: "invalid"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, err := w.Write([]byte(tt.responseBody))
				require.NoError(t, err)
			}))
			defer server.Close()

			client := NewAuthorizationClient(server.URL, server.Client())
			_, err := client.ListOwnProjectAuthorizations(context.Background(), bearerToken, "user-1", "project-1")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.NotContains(t, err.Error(), bearerToken)
			assert.NotContains(t, err.Error(), tt.responseBody)
			assert.NotContains(t, err.Error(), "secret-response-body")
			assert.NotContains(t, err.Error(), "secret-malformed-response-body")
		})
	}
}

func TestAuthorizationClientRejectsOversizedResponseBeforeDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "response is too large")
}

func TestAuthorizationClientFailsClosedAboveOneThousandReturnedAuthorizations(t *testing.T) {
	server := newAuthorizationListServer(t, 1001, []map[string]any{
		authorizationFixture("auth-1", "user-1", "project-1", "org-1", "Organization 1", "STATE_ACTIVE", "viewer"),
	})
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1,000")
}

func TestAuthorizationClientRejectsPaginationThatMakesNoProgress(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeAuthorizationListResponse(t, w, 2, []map[string]any{
				authorizationFixture("auth-1", "user-1", "project-1", "org-1", "Organization 1", "STATE_ACTIVE", "viewer"),
			})
			return
		}
		writeAuthorizationListResponse(t, w, 2, []map[string]any{})
	}))
	defer server.Close()

	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListOwnProjectAuthorizations(context.Background(), "token", "user-1", "project-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagination")
}

func TestAuthorizationClientRejectsBlankRequiredInputsBeforeSendingRequest(t *testing.T) {
	testCases := []struct {
		name      string
		baseURL   string
		token     string
		subject   string
		projectID string
	}{
		{name: "API URL", token: "token", subject: "user-1", projectID: "project-1"},
		{name: "bearer token", baseURL: "https://zitadel.invalid", subject: "user-1", projectID: "project-1"},
		{name: "subject", baseURL: "https://zitadel.invalid", token: "token", projectID: "project-1"},
		{name: "project id", baseURL: "https://zitadel.invalid", token: "token", subject: "user-1"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAuthorizationClient(tt.baseURL, http.DefaultClient)
			_, err := client.ListOwnProjectAuthorizations(context.Background(), tt.token, tt.subject, tt.projectID)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.name))
		})
	}
}

func newAuthorizationListServer(t *testing.T, total int, authorizations []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAuthorizationListResponse(t, w, total, authorizations)
	}))
}

func writeAuthorizationListResponse(t *testing.T, w http.ResponseWriter, total int, authorizations []map[string]any) {
	t.Helper()
	writeAuthorizationListResponseValue(t, w, strconv.Itoa(total), authorizations)
}

func writeAuthorizationListResponseValue(t *testing.T, w http.ResponseWriter, total any, authorizations []map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"pagination": map[string]any{
			"totalResult":  total,
			"appliedLimit": 100,
		},
		"authorizations": authorizations,
	}))
}

func authorizationFixture(id, userID, projectID, organizationID, organizationName, state string, roles ...string) map[string]any {
	roleValues := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		roleValues = append(roleValues, map[string]any{
			"key":         role,
			"displayName": fmt.Sprintf("%s display name", role),
			"group":       "ListingKit",
		})
	}
	return map[string]any{
		"id": id,
		"project": map[string]any{
			"id":             projectID,
			"name":           "Synthetic ListingKit",
			"organizationId": "provider-org",
		},
		"organization": map[string]any{
			"id":   organizationID,
			"name": organizationName,
		},
		"user": map[string]any{
			"id":                 userID,
			"preferredLoginName": "user@example.invalid",
			"displayName":        "Synthetic User",
			"organizationId":     "home-org",
		},
		"state": state,
		"roles": roleValues,
	}
}
