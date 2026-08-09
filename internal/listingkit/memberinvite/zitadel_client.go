package memberinvite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	createHumanUserPath     = "/v2/users/human"
	createAuthorizationPath = "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization"
)

type ZitadelConfig struct {
	IssuerURL  string
	Token      string
	ProjectID  string
	HTTPClient *http.Client
}

type zitadelProvider struct {
	issuerURL string
	token     string
	projectID string
	http      *http.Client
}

func NewZitadelProvider(cfg ZitadelConfig) (Provider, error) {
	issuerURL := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	token := strings.TrimSpace(cfg.Token)
	projectID := strings.TrimSpace(cfg.ProjectID)
	if issuerURL == "" || token == "" || projectID == "" {
		return nil, errors.New("incomplete ZITADEL member invitation configuration")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &zitadelProvider{issuerURL: issuerURL, token: token, projectID: projectID, http: client}, nil
}

func (p *zitadelProvider) Invite(ctx context.Context, request InviteRequest) (Invitation, error) {
	request, err := normalizeRequest(request)
	if err != nil {
		return Invitation{}, err
	}
	userID, err := p.createHumanUser(ctx, request)
	if err != nil {
		return Invitation{}, err
	}
	authorizationID, err := p.createAuthorization(ctx, request.TenantID, userID, request.Role)
	if err != nil {
		return Invitation{}, newIncompleteError(userID, err)
	}
	return Invitation{
		TenantID:        request.TenantID,
		UserID:          userID,
		Email:           request.Email,
		Role:            request.Role,
		AuthorizationID: authorizationID,
	}, nil
}

func (p *zitadelProvider) createHumanUser(ctx context.Context, request InviteRequest) (string, error) {
	body := struct {
		Organization struct {
			OrganizationID string `json:"orgId"`
		} `json:"organization"`
		Profile struct {
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
		} `json:"profile"`
		Email struct {
			Email    string   `json:"email"`
			SendCode struct{} `json:"sendCode"`
		} `json:"email"`
	}{}
	body.Organization.OrganizationID = request.TenantID
	body.Profile.GivenName = request.GivenName
	body.Profile.FamilyName = request.FamilyName
	body.Email.Email = request.Email

	var response struct {
		UserID string `json:"userId"`
		ID     string `json:"id"`
	}
	if err := p.doJSON(ctx, createHumanUserPath, body, &response); err != nil {
		return "", err
	}
	if response.UserID == "" {
		response.UserID = response.ID
	}
	if response.UserID == "" {
		return "", errors.New("ZITADEL user creation did not return a user id")
	}
	return response.UserID, nil
}

func (p *zitadelProvider) createAuthorization(ctx context.Context, organizationID, userID, role string) (string, error) {
	body := struct {
		UserID         string   `json:"userId"`
		ProjectID      string   `json:"projectId"`
		OrganizationID string   `json:"organizationId"`
		RoleKeys       []string `json:"roleKeys"`
	}{
		UserID: userID, ProjectID: p.projectID, OrganizationID: organizationID, RoleKeys: []string{role},
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := p.doJSON(ctx, createAuthorizationPath, body, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("ZITADEL role assignment did not return an authorization id")
	}
	return response.ID, nil
}

func (p *zitadelProvider) doJSON(ctx context.Context, path string, body, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.issuerURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+p.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if path == createAuthorizationPath {
		request.Header.Set("Connect-Protocol-Version", "1")
	}

	response, err := p.http.Do(request)
	if err != nil {
		return safeProviderError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return zitadelRequestError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return errors.New("invalid ZITADEL response")
	}
	return nil
}

func zitadelRequestError(response *http.Response) error {
	if response.StatusCode == http.StatusConflict {
		return ErrConflict
	}
	return &providerStatusError{statusCode: response.StatusCode}
}
