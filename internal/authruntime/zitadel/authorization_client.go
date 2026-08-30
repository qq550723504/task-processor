package zitadel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
)

const (
	authorizationListEndpoint      = "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations"
	authorizationListPageSize      = 100
	authorizationListMaximumResult = 1000
)

// AuthorizationClient reads the current user's organization-scoped ZITADEL
// role assignments using the official v2 AuthorizationService contract.
type AuthorizationClient struct {
	apiURL     string
	httpClient *http.Client
}

// NewAuthorizationClient creates a client for ZITADEL's v2 Authorization API.
func NewAuthorizationClient(apiURL string, httpClient *http.Client) *AuthorizationClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &AuthorizationClient{
		apiURL:     strings.TrimRight(strings.TrimSpace(apiURL), "/"),
		httpClient: httpClient,
	}
}

type authorizationListRequest struct {
	Pagination authorizationPaginationRequest `json:"pagination"`
	Sorting    string                         `json:"sortingColumn"`
	Filters    []authorizationSearchFilter    `json:"filters"`
}

type authorizationPaginationRequest struct {
	Offset int  `json:"offset"`
	Limit  int  `json:"limit"`
	Asc    bool `json:"asc"`
}

type authorizationSearchFilter struct {
	InUserIDs *authorizationIDsFilter `json:"inUserIds,omitempty"`
	ProjectID *authorizationIDFilter  `json:"projectId,omitempty"`
}

type authorizationIDsFilter struct {
	IDs []string `json:"ids"`
}

type authorizationIDFilter struct {
	ID string `json:"id"`
}

type authorizationListResponse struct {
	Pagination struct {
		TotalResult protoJSONUint64 `json:"totalResult"`
	} `json:"pagination"`
	Authorizations []authorizationRecordV2 `json:"authorizations"`
}

type authorizationRecordV2 struct {
	ID      string `json:"id"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	Organization struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"organization"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	State string `json:"state"`
	Roles []struct {
		Key string `json:"key"`
	} `json:"roles"`
}

type organizationGrantAccumulator struct {
	grant authidentity.OrganizationGrant
	roles map[string]struct{}
}

// protoJSONUint64 accepts the canonical quoted decimal representation emitted
// by ProtoJSON and unquoted integer values for compatibility with JSON proxies.
type protoJSONUint64 uint64

func (value *protoJSONUint64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return errors.New("empty ProtoJSON uint64")
	}

	var parsed uint64
	if data[0] == '"' {
		var decimal string
		if err := json.Unmarshal(data, &decimal); err != nil {
			return err
		}
		converted, err := strconv.ParseUint(decimal, 10, 64)
		if err != nil {
			return err
		}
		parsed = converted
	} else if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*value = protoJSONUint64(parsed)
	return nil
}

// ListOwnProjectAuthorizations returns only active role assignments belonging
// to subject and projectID. bearerToken is the user's access token, never a
// management credential.
func (c *AuthorizationClient) ListOwnProjectAuthorizations(
	ctx context.Context,
	bearerToken string,
	subject string,
	projectID string,
) ([]authidentity.OrganizationGrant, error) {
	if c == nil || strings.TrimSpace(c.apiURL) == "" {
		return nil, errors.New("ZITADEL authorization API URL is required")
	}
	bearerToken = strings.TrimSpace(bearerToken)
	if bearerToken == "" {
		return nil, errors.New("ZITADEL bearer token is required")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, errors.New("ZITADEL subject is required")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("ZITADEL project ID is required")
	}

	grants := make(map[string]*organizationGrantAccumulator)
	seenAuthorizationIDs := make(map[string]struct{})
	var firstTotalResult uint64
	firstPage := true
	for offset := 0; ; {
		response, err := c.listAuthorizationPage(ctx, bearerToken, subject, projectID, offset)
		if err != nil {
			return nil, err
		}
		totalResult := uint64(response.Pagination.TotalResult)
		if firstPage {
			firstTotalResult = totalResult
			firstPage = false
		} else if totalResult != firstTotalResult {
			return nil, errors.New("ZITADEL authorization pagination total result changed")
		}
		if firstTotalResult > authorizationListMaximumResult {
			return nil, errors.New("ZITADEL authorization query returned more than 1,000 authorizations")
		}

		for _, authorization := range response.Authorizations {
			authorizationID := strings.TrimSpace(authorization.ID)
			if authorizationID == "" {
				return nil, errors.New("ZITADEL authorization contains a blank authorization id")
			}
			if _, duplicate := seenAuthorizationIDs[authorizationID]; duplicate {
				return nil, errors.New("ZITADEL authorization pagination returned a duplicate authorization id")
			}
			seenAuthorizationIDs[authorizationID] = struct{}{}
			if err := addAuthorizationGrant(grants, authorization, subject, projectID); err != nil {
				return nil, err
			}
		}

		returned := uint64(len(seenAuthorizationIDs))
		if returned > authorizationListMaximumResult {
			return nil, errors.New("ZITADEL authorization query returned more than 1,000 authorizations")
		}
		if returned > firstTotalResult {
			return nil, errors.New("ZITADEL authorization pagination exceeded its total result")
		}
		if returned == firstTotalResult {
			return sortedOrganizationGrants(grants), nil
		}
		if len(response.Authorizations) == 0 {
			return nil, errors.New("ZITADEL authorization pagination made no progress")
		}
		offset = len(seenAuthorizationIDs)
	}
}

func (c *AuthorizationClient) listAuthorizationPage(
	ctx context.Context,
	bearerToken string,
	subject string,
	projectID string,
	offset int,
) (authorizationListResponse, error) {
	payload := authorizationListRequest{
		Pagination: authorizationPaginationRequest{
			Offset: offset,
			Limit:  authorizationListPageSize,
			Asc:    true,
		},
		Sorting: "AUTHORIZATION_FIELD_NAME_ID",
		Filters: []authorizationSearchFilter{
			{InUserIDs: &authorizationIDsFilter{IDs: []string{subject}}},
			{ProjectID: &authorizationIDFilter{ID: projectID}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return authorizationListResponse{}, errors.New("encode ZITADEL authorization request")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+authorizationListEndpoint, bytes.NewReader(body))
	if err != nil {
		return authorizationListResponse{}, fmt.Errorf("create ZITADEL authorization request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+bearerToken)
	request.Header.Set("Connect-Protocol-Version", "1")
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return authorizationListResponse{}, fmt.Errorf("ZITADEL authorization request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return authorizationListResponse{}, fmt.Errorf("ZITADEL authorization request failed: HTTP %d", response.StatusCode)
	}

	var result authorizationListResponse
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&result); err != nil {
		return authorizationListResponse{}, errors.New("ZITADEL authorization response is invalid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return authorizationListResponse{}, errors.New("ZITADEL authorization response is invalid JSON")
	}
	return result, nil
}

func addAuthorizationGrant(
	grants map[string]*organizationGrantAccumulator,
	authorization authorizationRecordV2,
	subject string,
	projectID string,
) error {
	organizationID := strings.TrimSpace(authorization.Organization.ID)
	if organizationID == "" {
		return errors.New("ZITADEL authorization contains a blank organization id")
	}
	authorizationProjectID := strings.TrimSpace(authorization.Project.ID)
	if authorizationProjectID == "" {
		return errors.New("ZITADEL authorization contains a blank project id")
	}
	userID := strings.TrimSpace(authorization.User.ID)
	if userID == "" {
		return errors.New("ZITADEL authorization contains a blank user id")
	}
	if authorization.State != "STATE_ACTIVE" || userID != subject || authorizationProjectID != projectID {
		return nil
	}

	accumulator, ok := grants[organizationID]
	if !ok {
		accumulator = &organizationGrantAccumulator{
			grant: authidentity.OrganizationGrant{
				OrganizationID:   organizationID,
				OrganizationName: strings.TrimSpace(authorization.Organization.Name),
				ProjectID:        authorizationProjectID,
			},
			roles: make(map[string]struct{}),
		}
		grants[organizationID] = accumulator
	} else if accumulator.grant.OrganizationName == "" {
		accumulator.grant.OrganizationName = strings.TrimSpace(authorization.Organization.Name)
	}
	for _, role := range authorization.Roles {
		if key := strings.TrimSpace(role.Key); key != "" {
			accumulator.roles[key] = struct{}{}
		}
	}
	return nil
}

func sortedOrganizationGrants(accumulators map[string]*organizationGrantAccumulator) []authidentity.OrganizationGrant {
	organizationIDs := make([]string, 0, len(accumulators))
	for organizationID := range accumulators {
		organizationIDs = append(organizationIDs, organizationID)
	}
	sort.Strings(organizationIDs)

	grants := make([]authidentity.OrganizationGrant, 0, len(organizationIDs))
	for _, organizationID := range organizationIDs {
		accumulator := accumulators[organizationID]
		roles := make([]string, 0, len(accumulator.roles))
		for role := range accumulator.roles {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		accumulator.grant.Roles = roles
		grants = append(grants, accumulator.grant)
	}
	return grants
}
