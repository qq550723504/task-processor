package zitadelprovision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Config struct {
	IssuerURL       string
	ManagementToken string
	OrgID           string
	ProjectID       string
	ProjectName     string
	CreateProject   bool
	HTTPClient      *http.Client
}

type ProjectRole struct {
	Key         string
	DisplayName string
	Group       string
}

type RoleResult struct {
	Role    ProjectRole
	Existed bool
}

type Result struct {
	ProjectID         string
	ProjectName       string
	Roles             []RoleResult
	RecommendedScopes []string
	AllowedRoles      []string
}

func DefaultRoles() []ProjectRole {
	return []ProjectRole{
		{Key: "listingkit_viewer", DisplayName: "ListingKit Viewer", Group: "ListingKit"},
		{Key: "listingkit_operator", DisplayName: "ListingKit Operator", Group: "ListingKit"},
		{Key: "listingkit_admin", DisplayName: "ListingKit Admin", Group: "ListingKit"},
		{Key: "platform_admin", DisplayName: "Platform Admin", Group: "ListingKit"},
	}
}

func RecommendedScopes() []string {
	scopes := []string{
		"openid",
		"profile",
		"email",
		"urn:zitadel:iam:user:resourceowner",
		"urn:zitadel:iam:org:project:id:zitadel:aud",
	}
	for _, role := range DefaultRoles() {
		scopes = append(scopes, "urn:zitadel:iam:org:project:role:"+role.Key)
	}
	return scopes
}

func Provision(ctx context.Context, cfg Config) (Result, error) {
	if err := cfg.validate(); err != nil {
		return Result{}, err
	}
	client := newClient(cfg)
	projectID := cfg.ProjectID
	projectName := strings.TrimSpace(cfg.ProjectName)
	if projectName == "" {
		projectName = "ListingKit"
	}

	if projectID == "" {
		foundID, err := client.findProject(ctx, projectName)
		if err != nil {
			return Result{}, err
		}
		projectID = foundID
	}
	if projectID == "" {
		if !cfg.CreateProject {
			return Result{}, fmt.Errorf("project %s not found; pass -create-project to create it", projectName)
		}
		createdID, err := client.createProject(ctx, projectName)
		if err != nil {
			return Result{}, err
		}
		projectID = createdID
	}

	existingRoles, err := client.listProjectRoles(ctx, projectID)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		ProjectID:         projectID,
		ProjectName:       projectName,
		RecommendedScopes: RecommendedScopes(),
		AllowedRoles:      roleKeys(DefaultRoles()),
	}
	for _, role := range DefaultRoles() {
		_, existed := existingRoles[role.Key]
		if !existed {
			if err := client.createProjectRole(ctx, projectID, role); err != nil {
				return Result{}, err
			}
		}
		result.Roles = append(result.Roles, RoleResult{
			Role:    role,
			Existed: existed,
		})
	}
	return result, nil
}

func (cfg Config) validate() error {
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return errors.New("issuer URL is required")
	}
	if strings.TrimSpace(cfg.ManagementToken) == "" {
		return errors.New("management token is required")
	}
	return nil
}

type client struct {
	baseURL string
	token   string
	orgID   string
	http    *http.Client
}

func newClient(cfg Config) client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return client{
		baseURL: strings.TrimRight(cfg.IssuerURL, "/"),
		token:   strings.TrimSpace(cfg.ManagementToken),
		orgID:   strings.TrimSpace(cfg.OrgID),
		http:    httpClient,
	}
}

func (c client) findProject(ctx context.Context, name string) (string, error) {
	var response struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/_search", map[string]any{
		"queries": []map[string]any{
			{
				"nameQuery": map[string]any{
					"name":   name,
					"method": "TEXT_QUERY_METHOD_EQUALS",
				},
			},
		},
	}, &response); err != nil {
		return "", err
	}
	for _, project := range response.Result {
		if project.Name == name {
			return project.ID, nil
		}
	}
	return "", nil
}

func (c client) createProject(ctx context.Context, name string) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects", map[string]any{
		"name":                 name,
		"projectRoleAssertion": true,
		"projectRoleCheck":     true,
		"hasProjectCheck":      true,
	}, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("ZITADEL create project response did not include an id")
	}
	return response.ID, nil
}

func (c client) listProjectRoles(ctx context.Context, projectID string) (map[string]ProjectRole, error) {
	var response struct {
		Result []struct {
			Key         string `json:"key"`
			DisplayName string `json:"displayName"`
			Group       string `json:"group"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(projectID)+"/roles/_search", map[string]any{}, &response); err != nil {
		return nil, err
	}
	roles := make(map[string]ProjectRole, len(response.Result))
	for _, role := range response.Result {
		if role.Key == "" {
			continue
		}
		roles[role.Key] = ProjectRole{
			Key:         role.Key,
			DisplayName: role.DisplayName,
			Group:       role.Group,
		}
	}
	return roles, nil
}

func (c client) createProjectRole(ctx context.Context, projectID string, role ProjectRole) error {
	var response map[string]any
	return c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(projectID)+"/roles", map[string]any{
		"roleKey":     role.Key,
		"displayName": role.DisplayName,
		"group":       role.Group,
	}, &response)
}

func (c client) doJSON(ctx context.Context, method string, path string, body any, target any) error {
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &payload)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if c.orgID != "" {
		request.Header.Set("x-zitadel-orgid", c.orgID)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return fmt.Errorf("ZITADEL %s %s failed: %s", method, path, response.Status)
		}
		return fmt.Errorf("ZITADEL %s %s failed: %s: %s", method, path, response.Status, detail)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func roleKeys(roles []ProjectRole) []string {
	keys := make([]string, 0, len(roles))
	for _, role := range roles {
		keys = append(keys, role.Key)
	}
	return keys
}
