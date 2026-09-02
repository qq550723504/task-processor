package zitadelprovision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/zitadelprotojson"
)

type Config struct {
	IssuerURL       string
	ManagementToken string
	OrgID           string
	ProjectID       string
	ProjectName     string
	CreateProject   bool
	// HasProjectCheck controls whether ZITADEL requires an organization-level
	// grant before users can authenticate to the project. Nil preserves the
	// existing provisioning default.
	HasProjectCheck *bool
	// BootstrapLoginName identifies the local human account that should receive
	// the operator role before the first browser login. Empty disables bootstrap.
	BootstrapLoginName string
	// AcceptanceOrganizationIDs are stable, provisioner-owned IDs for the two
	// disposable local acceptance organizations. Name-only adoption is unsafe.
	AcceptanceOrganizationIDs []string
	HTTPClient                *http.Client
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

type LocalApplicationConfig struct {
	APIName                string
	OIDCName               string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	RotateAPIClientSecret  bool
	RotateOIDCClientSecret bool
}

type LocalApplicationResult struct {
	ProjectID         string
	APIAppID          string
	APIClientID       string
	APIClientSecret   string
	OIDCAppID         string
	OIDCClientID      string
	OIDCClientSecret  string
	BootstrapTenantID string
	BootstrapUserID   string
	RecommendedScopes []string
}

type MultiOrganizationAcceptanceSpec struct {
	UserID        string
	Organizations []AcceptanceOrganizationSpec
}

type AcceptanceOrganizationSpec struct {
	Name     string
	RoleKeys []string
}

type AcceptanceOrganizationResult struct {
	OrganizationID   string
	OrganizationName string
	RoleKeys         []string
}

type MultiOrganizationAcceptanceResult struct {
	UserID        string
	ProjectID     string
	Organizations []AcceptanceOrganizationResult
}

// String intentionally omits generated client secrets. Results are commonly
// formatted by CLI callers and must remain safe to print.
func (r LocalApplicationResult) String() string {
	return fmt.Sprintf("LocalApplicationResult{ProjectID:%q APIAppID:%q APIClientID:%q OIDCAppID:%q OIDCClientID:%q RecommendedScopes:%v}",
		r.ProjectID, r.APIAppID, r.APIClientID, r.OIDCAppID, r.OIDCClientID, r.RecommendedScopes)
}

func DefaultRoles() []ProjectRole {
	return []ProjectRole{
		{Key: "listingkit_viewer", DisplayName: "ListingKit Viewer", Group: "ListingKit"},
		{Key: "listingkit_operator", DisplayName: "ListingKit Operator", Group: "ListingKit"},
		{Key: "listingkit_admin", DisplayName: "ListingKit Admin", Group: "ListingKit"},
		{Key: "platform_admin", DisplayName: "Platform Admin", Group: "ListingKit"},
	}
}

func RecommendedScopes(projectID string) []string {
	scopes := []string{
		"openid",
		"profile",
		"email",
		"urn:zitadel:iam:user:resourceowner",
		"urn:zitadel:iam:org:project:id:zitadel:aud",
	}
	if projectID = strings.TrimSpace(projectID); projectID != "" {
		scopes = append(scopes,
			"urn:zitadel:iam:org:project:id:"+projectID+":aud",
			"urn:zitadel:iam:org:project:"+projectID+":roles",
		)
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
	createdProject := false
	if projectID == "" {
		if !cfg.CreateProject {
			return Result{}, fmt.Errorf("project %s not found; pass -create-project to create it", projectName)
		}
		createdID, err := client.createProject(ctx, projectName, cfg.HasProjectCheck)
		if err != nil {
			return Result{}, err
		}
		projectID = createdID
		createdProject = true
	}
	if cfg.HasProjectCheck != nil && !createdProject {
		if err := client.updateProject(ctx, projectID, projectName, *cfg.HasProjectCheck); err != nil {
			return Result{}, err
		}
	}

	existingRoles, err := client.listProjectRoles(ctx, projectID)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		ProjectID:         projectID,
		ProjectName:       projectName,
		RecommendedScopes: RecommendedScopes(projectID),
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

func ProvisionLocalApplications(ctx context.Context, cfg Config, appCfg LocalApplicationConfig) (LocalApplicationResult, error) {
	if err := cfg.validate(); err != nil {
		return LocalApplicationResult{}, err
	}
	if err := validateLocalIssuer(cfg.IssuerURL); err != nil {
		return LocalApplicationResult{}, err
	}
	appCfg.APIName = strings.TrimSpace(appCfg.APIName)
	appCfg.OIDCName = strings.TrimSpace(appCfg.OIDCName)
	if appCfg.APIName == "" || appCfg.OIDCName == "" {
		return LocalApplicationResult{}, errors.New("local API and OIDC application names are required")
	}
	if len(appCfg.RedirectURIs) == 0 || len(appCfg.PostLogoutRedirectURIs) == 0 {
		return LocalApplicationResult{}, errors.New("local OIDC redirect URIs are required")
	}
	if !equalStrings(appCfg.RedirectURIs, []string{"http://localhost:3000/api/auth/callback/zitadel"}) ||
		!equalStrings(appCfg.PostLogoutRedirectURIs, []string{"http://localhost:3000"}) {
		return LocalApplicationResult{}, errors.New("local OIDC redirects must use the fixed localhost acceptance URLs")
	}
	provisioned, err := Provision(ctx, cfg)
	if err != nil {
		return LocalApplicationResult{}, err
	}
	client := newClient(cfg)
	result := LocalApplicationResult{ProjectID: provisioned.ProjectID, RecommendedScopes: provisioned.RecommendedScopes}
	if loginName := strings.TrimSpace(cfg.BootstrapLoginName); loginName != "" {
		identity, err := bootstrapLocalOperator(ctx, cfg, loginName, provisioned.ProjectID)
		if err != nil {
			return LocalApplicationResult{}, err
		}
		result.BootstrapTenantID = identity.TenantID
		result.BootstrapUserID = identity.UserID
	}
	applications, err := client.listApplications(ctx, provisioned.ProjectID)
	if err != nil {
		return LocalApplicationResult{}, err
	}
	apiApp, ok, err := findApplicationByType(applications, appCfg.APIName, applicationAPI)
	if err != nil {
		return LocalApplicationResult{}, err
	}
	if ok {
		apiApp, err = client.getApplication(ctx, provisioned.ProjectID, apiApp.ID)
		if err != nil {
			return LocalApplicationResult{}, err
		}
		if apiApp.APIConfig == nil || apiApp.OIDCConfig != nil {
			return LocalApplicationResult{}, fmt.Errorf("local API application %q has an unexpected application type", appCfg.APIName)
		}
		rotateAPISecret := appCfg.RotateAPIClientSecret
		if apiApp.APIConfig.AuthMethodType == "" {
			// ZITADEL v4 omits the enum's default Basic value on some v1
			// responses. Requiring a newly generated client secret proves that
			// the reused local app still supports the Basic-secret contract.
			rotateAPISecret = true
		} else if apiApp.APIConfig.AuthMethodType != "API_AUTH_METHOD_TYPE_BASIC" {
			if err := client.updateAPIApplicationConfig(ctx, provisioned.ProjectID, apiApp.ID); err != nil {
				return LocalApplicationResult{}, err
			}
			apiApp, err = client.getApplication(ctx, provisioned.ProjectID, apiApp.ID)
			if err != nil {
				return LocalApplicationResult{}, err
			}
			if apiApp.APIConfig == nil || apiApp.APIConfig.AuthMethodType != "API_AUTH_METHOD_TYPE_BASIC" {
				return LocalApplicationResult{}, fmt.Errorf("local API application %q must use Basic authentication", appCfg.APIName)
			}
			rotateAPISecret = true
		}
		if rotateAPISecret {
			secret, err := client.regenerateAPIClientSecret(ctx, provisioned.ProjectID, apiApp.ID)
			if err != nil {
				return LocalApplicationResult{}, err
			}
			apiApp.APIConfig.ClientSecret = secret
		}
	}
	if !ok {
		apiApp, err = client.createAPIApplication(ctx, provisioned.ProjectID, appCfg.APIName)
		if err != nil {
			return LocalApplicationResult{}, err
		}
	}
	result.APIAppID = apiApp.ID
	result.APIClientID = apiApp.clientID()
	result.APIClientSecret = apiApp.clientSecret()
	if result.APIAppID == "" || result.APIClientID == "" {
		return LocalApplicationResult{}, errors.New("ZITADEL API application response did not include app and client ids")
	}

	oidcApp, ok, err := findApplicationByType(applications, appCfg.OIDCName, applicationOIDC)
	if err != nil {
		return LocalApplicationResult{}, err
	}
	if !ok {
		oidcApp, err = client.createOIDCApplication(ctx, provisioned.ProjectID, appCfg)
		if err != nil {
			return LocalApplicationResult{}, err
		}
	} else {
		updated, validateErr := validateExistingOIDCApplication(ctx, client, provisioned.ProjectID, oidcApp, appCfg)
		if validateErr != nil {
			return LocalApplicationResult{}, validateErr
		}
		if updated {
			appCfg.RotateOIDCClientSecret = true
		}
	}
	if ok && appCfg.RotateOIDCClientSecret {
		secret, err := client.regenerateOIDCClientSecret(ctx, provisioned.ProjectID, oidcApp.ID)
		if err != nil {
			return LocalApplicationResult{}, err
		}
		oidcApp.OIDCConfig.ClientSecret = secret
	}
	result.OIDCAppID = oidcApp.ID
	result.OIDCClientID = oidcApp.clientID()
	result.OIDCClientSecret = oidcApp.clientSecret()
	if result.OIDCAppID == "" || result.OIDCClientID == "" {
		return LocalApplicationResult{}, errors.New("ZITADEL OIDC application response did not include app and client ids")
	}
	return result, nil
}

func ProvisionLocalMultiOrganizationAcceptance(ctx context.Context, cfg Config, spec MultiOrganizationAcceptanceSpec) (MultiOrganizationAcceptanceResult, error) {
	if err := cfg.validate(); err != nil {
		return MultiOrganizationAcceptanceResult{}, err
	}
	if err := validateLocalIssuer(cfg.IssuerURL); err != nil {
		return MultiOrganizationAcceptanceResult{}, err
	}
	if cfg.HTTPClient == nil {
		httpClient, err := NewLoopbackOnlyHTTPClient(cfg.IssuerURL)
		if err != nil {
			return MultiOrganizationAcceptanceResult{}, err
		}
		cfg.HTTPClient = httpClient
	}
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return MultiOrganizationAcceptanceResult{}, errors.New("project id is required for multi-organization acceptance")
	}
	normalizedSpec, err := normalizeMultiOrganizationAcceptanceSpec(spec)
	if err != nil {
		return MultiOrganizationAcceptanceResult{}, err
	}
	acceptanceOrganizationIDs := normalizeStrings(cfg.AcceptanceOrganizationIDs)
	if len(acceptanceOrganizationIDs) != 2 {
		return MultiOrganizationAcceptanceResult{}, errors.New("exactly two distinct stable acceptance organization ids are required")
	}

	client := newClient(cfg)
	organizationIDs := make(map[string]string, len(normalizedSpec.Organizations))
	for index, requested := range normalizedSpec.Organizations {
		preferredID := acceptanceOrganizationIDs[index]
		organizationID, ensureErr := client.ensureAcceptanceOrganization(ctx, preferredID, requested.Name)
		if ensureErr != nil {
			return MultiOrganizationAcceptanceResult{}, ensureErr
		}
		organizationIDs[requested.Name] = organizationID
	}

	for _, requested := range normalizedSpec.Organizations {
		organizationID := organizationIDs[requested.Name]
		if ensureErr := client.ensureProjectGrant(ctx, projectID, organizationID, requested.RoleKeys); ensureErr != nil {
			return MultiOrganizationAcceptanceResult{}, ensureErr
		}
	}

	result := MultiOrganizationAcceptanceResult{UserID: normalizedSpec.UserID, ProjectID: projectID}
	for _, requested := range normalizedSpec.Organizations {
		organizationID := organizationIDs[requested.Name]
		if ensureErr := client.ensureAuthorization(ctx, normalizedSpec.UserID, projectID, organizationID, requested.RoleKeys); ensureErr != nil {
			return MultiOrganizationAcceptanceResult{}, ensureErr
		}
		result.Organizations = append(result.Organizations, AcceptanceOrganizationResult{
			OrganizationID:   organizationID,
			OrganizationName: requested.Name,
			RoleKeys:         append([]string(nil), requested.RoleKeys...),
		})
	}
	for _, requested := range normalizedSpec.Organizations {
		organizationID := organizationIDs[requested.Name]
		if verifyErr := client.verifyAcceptanceState(ctx, normalizedSpec.UserID, projectID, organizationID, requested.Name, requested.RoleKeys); verifyErr != nil {
			return MultiOrganizationAcceptanceResult{}, verifyErr
		}
	}
	return result, nil
}

func normalizeMultiOrganizationAcceptanceSpec(spec MultiOrganizationAcceptanceSpec) (MultiOrganizationAcceptanceSpec, error) {
	normalized := MultiOrganizationAcceptanceSpec{UserID: strings.TrimSpace(spec.UserID)}
	if normalized.UserID == "" {
		return MultiOrganizationAcceptanceSpec{}, errors.New("user id is required for multi-organization acceptance")
	}
	organizationsByName := make(map[string]int, len(spec.Organizations))
	for _, organization := range spec.Organizations {
		name := strings.TrimSpace(organization.Name)
		if name == "" {
			continue
		}
		roleKeys := normalizeStrings(organization.RoleKeys)
		if index, found := organizationsByName[name]; found {
			normalized.Organizations[index].RoleKeys = mergeUniqueStrings(normalized.Organizations[index].RoleKeys, roleKeys)
			continue
		}
		organizationsByName[name] = len(normalized.Organizations)
		normalized.Organizations = append(normalized.Organizations, AcceptanceOrganizationSpec{Name: name, RoleKeys: roleKeys})
	}
	if len(normalized.Organizations) != 2 {
		return MultiOrganizationAcceptanceSpec{}, errors.New("exactly two distinct organization names are required for multi-organization acceptance")
	}
	for _, organization := range normalized.Organizations {
		if len(organization.RoleKeys) == 0 {
			return MultiOrganizationAcceptanceSpec{}, errors.New("each acceptance organization requires at least one role key")
		}
	}
	return normalized, nil
}

func normalizeStrings(values []string) []string {
	return mergeUniqueStrings(nil, values)
}

func mergeUniqueStrings(existing, additions []string) []string {
	result := append([]string(nil), existing...)
	seen := make(map[string]struct{}, len(result)+len(additions))
	for _, value := range result {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func bootstrapLocalOperator(ctx context.Context, cfg Config, loginName, projectID string) (authidentity.AuthenticatedIdentity, error) {
	user, err := newClient(cfg).findGlobalUserByLoginName(ctx, loginName)
	if err != nil {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("find local bootstrap user %q: %w", loginName, err)
	}
	if user.ID == "" || user.Details.ResourceOwner == "" {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("local bootstrap user %q response is missing id or resource owner", loginName)
	}
	identity := authidentity.AuthenticatedIdentity{TenantID: user.Details.ResourceOwner, UserID: user.ID}
	bootstrapConfig := cfg
	bootstrapConfig.ProjectID = projectID
	if err := GrantLocalOperator(ctx, bootstrapConfig, "", identity); err != nil {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("grant local bootstrap operator: %w", err)
	}
	return identity, nil
}

func GrantLocalOperator(ctx context.Context, cfg Config, additionalRole string, identity authidentity.AuthenticatedIdentity) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := validateLocalIssuer(cfg.IssuerURL); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return errors.New("project id is required for local operator grant")
	}
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return errors.New("verified tenant and user identity are required for local operator grant")
	}
	additionalRole = strings.TrimSpace(additionalRole)
	if additionalRole != "" && additionalRole != "listingkit_admin" {
		return fmt.Errorf("unsupported local additional role %q", additionalRole)
	}
	client := newClient(cfg)
	grant, found, err := client.findUserGrant(ctx, cfg.ProjectID, identity)
	if err != nil {
		return err
	}
	if found {
		roles := append([]string(nil), grant.RoleKeys...)
		if !containsString(roles, "listingkit_operator") {
			roles = append(roles, "listingkit_operator")
		}
		if additionalRole != "" && !containsString(roles, additionalRole) {
			roles = append(roles, additionalRole)
		}
		if equalStrings(roles, grant.RoleKeys) {
			return nil
		}
		return client.updateAuthorization(ctx, grant.ID, roles)
	}
	roles := []string{"listingkit_operator"}
	if additionalRole != "" {
		roles = append(roles, additionalRole)
	}
	return client.createAuthorization(ctx, cfg.ProjectID, identity, roles)
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

type globalUserRecord struct {
	ID            string `json:"id"`
	PreferredName string `json:"preferredLoginName"`
	Details       struct {
		ResourceOwner string `json:"resourceOwner"`
	} `json:"details"`
}

func (c client) findGlobalUserByLoginName(ctx context.Context, loginName string) (globalUserRecord, error) {
	var response struct {
		User globalUserRecord `json:"user"`
	}
	path := "/management/v1/global/users/_by_login_name?loginName=" + url.QueryEscape(loginName)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return globalUserRecord{}, err
	}
	return response.User, nil
}

type applicationKind string

const (
	applicationAPI  applicationKind = "API"
	applicationOIDC applicationKind = "OIDC"
)

type apiApplicationConfig struct {
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	AuthMethodType string `json:"authMethodType"`
}

type oidcApplicationConfig struct {
	ClientID                 string   `json:"clientId"`
	ClientSecret             string   `json:"clientSecret"`
	RedirectURIs             []string `json:"redirectUris"`
	ResponseTypes            []string `json:"responseTypes"`
	GrantTypes               []string `json:"grantTypes"`
	AppType                  string   `json:"appType"`
	AuthMethodType           string   `json:"authMethodType"`
	PostLogoutRedirectURIs   []string `json:"postLogoutRedirectUris"`
	DevMode                  *bool    `json:"devMode"`
	AccessTokenType          string   `json:"accessTokenType"`
	AccessTokenRoleAssertion *bool    `json:"accessTokenRoleAssertion"`
	IDTokenRoleAssertion     *bool    `json:"idTokenRoleAssertion"`
}

type applicationRecord struct {
	ID         string                 `json:"id"`
	AppID      string                 `json:"appId"`
	Name       string                 `json:"name"`
	APIConfig  *apiApplicationConfig  `json:"apiConfig"`
	OIDCConfig *oidcApplicationConfig `json:"oidcConfig"`
}

func (a applicationRecord) clientID() string {
	if a.APIConfig != nil && a.APIConfig.ClientID != "" {
		return a.APIConfig.ClientID
	}
	if a.OIDCConfig != nil {
		return a.OIDCConfig.ClientID
	}
	return ""
}

func (a applicationRecord) clientSecret() string {
	if a.APIConfig != nil && a.APIConfig.ClientSecret != "" {
		return a.APIConfig.ClientSecret
	}
	if a.OIDCConfig != nil {
		return a.OIDCConfig.ClientSecret
	}
	return ""
}

func (a applicationRecord) appID() string {
	if a.ID != "" {
		return a.ID
	}
	return a.AppID
}

func (a applicationRecord) normalized() applicationRecord {
	a.ID = a.appID()
	return a
}

func findApplication(applications []applicationRecord, name string) (applicationRecord, bool) {
	for _, application := range applications {
		if strings.TrimSpace(application.Name) == name {
			return application.normalized(), true
		}
	}
	return applicationRecord{}, false
}

func findApplicationByType(applications []applicationRecord, name string, kind applicationKind) (applicationRecord, bool, error) {
	for _, application := range applications {
		if strings.TrimSpace(application.Name) != name {
			continue
		}
		application = application.normalized()
		switch kind {
		case applicationAPI:
			if application.APIConfig == nil || application.OIDCConfig != nil {
				return applicationRecord{}, false, fmt.Errorf("local API application name %q is already used by a different application type", name)
			}
		case applicationOIDC:
			if application.OIDCConfig == nil || application.APIConfig != nil {
				return applicationRecord{}, false, fmt.Errorf("local OIDC application name %q is already used by a different application type", name)
			}
		default:
			return applicationRecord{}, false, errors.New("unsupported local application type")
		}
		return application, true, nil
	}
	return applicationRecord{}, false, nil
}

func validateLocalIssuer(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("local ZITADEL issuer URL is invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("local ZITADEL issuer URL must use localhost, 127.0.0.1, or ::1")
	}
	return nil
}

type lookupIPFunc func(context.Context, string, string) ([]net.IP, error)
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

const loopbackHTTPClientTimeout = 5 * time.Second

func NewLoopbackOnlyHTTPClient(raw string) (*http.Client, error) {
	return newLoopbackOnlyHTTPClient(raw, net.DefaultResolver.LookupIP)
}

func newLoopbackOnlyHTTPClient(raw string, lookup lookupIPFunc) (*http.Client, error) {
	dialer := &net.Dialer{}
	return newLoopbackOnlyHTTPClientWithDialer(raw, lookup, dialer.DialContext)
}

func newLoopbackOnlyHTTPClientWithDialer(raw string, lookup lookupIPFunc, dial dialContextFunc) (*http.Client, error) {
	if err := validateLocalIssuer(raw); err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(strings.TrimSpace(raw))
	if _, err := resolveLoopbackHost(context.Background(), parsed.Hostname(), lookup); err != nil {
		return nil, err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("loopback HTTP destination is invalid")
		}
		ips, err := resolveLoopbackHost(ctx, host, lookup)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}}
	return &http.Client{
		Transport: transport,
		Timeout:   loopbackHTTPClientTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func resolveLoopbackHost(ctx context.Context, host string, lookup lookupIPFunc) ([]net.IP, error) {
	host = strings.ToLower(host)
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, errors.New("HTTP destination hostname is not an allowed loopback name")
	}
	var ips []net.IP
	if literal := net.ParseIP(host); literal != nil {
		ips = []net.IP{literal}
	} else {
		resolved, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, errors.New("resolve loopback issuer hostname")
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return nil, errors.New("loopback issuer hostname resolved to no addresses")
	}
	for _, ip := range ips {
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("loopback issuer hostname resolved to a non-loopback address")
		}
	}
	return ips, nil
}

func validateExistingOIDCApplication(ctx context.Context, client client, projectID string, application applicationRecord, cfg LocalApplicationConfig) (bool, error) {
	application, err := client.getApplication(ctx, projectID, application.ID)
	if err != nil {
		return false, err
	}
	if application.OIDCConfig == nil || application.APIConfig != nil {
		return false, errors.New("existing local OIDC application has an unexpected application type")
	}
	config := application.OIDCConfig
	if equalStrings(config.RedirectURIs, cfg.RedirectURIs) &&
		equalStrings(config.PostLogoutRedirectURIs, cfg.PostLogoutRedirectURIs) &&
		equalStrings(config.ResponseTypes, []string{"OIDC_RESPONSE_TYPE_CODE"}) &&
		equalStrings(config.GrantTypes, []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"}) &&
		(config.AppType == "" || config.AppType == "OIDC_APP_TYPE_WEB") &&
		(config.AuthMethodType == "" || config.AuthMethodType == "OIDC_AUTH_METHOD_TYPE_BASIC") &&
		(config.AccessTokenType == "" || config.AccessTokenType == "OIDC_TOKEN_TYPE_BEARER") &&
		config.DevMode != nil && *config.DevMode &&
		config.AccessTokenRoleAssertion != nil && *config.AccessTokenRoleAssertion &&
		config.IDTokenRoleAssertion != nil && *config.IDTokenRoleAssertion {
		// ZITADEL v4 omits the default Web/Basic enums in some v1 responses.
		// A regenerated client secret is the fail-closed proof that the reused
		// application still implements the confidential Basic contract.
		return config.AppType == "" || config.AuthMethodType == "", nil
	}
	if err := client.updateOIDCApplicationConfig(ctx, projectID, application.ID, cfg); err != nil {
		return false, err
	}
	return true, nil
}

func (c client) listApplications(ctx context.Context, projectID string) ([]applicationRecord, error) {
	var response struct {
		Result []applicationRecord `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(projectID)+"/apps/_search", map[string]any{}, &response); err != nil {
		return nil, err
	}
	for index := range response.Result {
		response.Result[index] = response.Result[index].normalized()
	}
	return response.Result, nil
}

func (c client) getApplication(ctx context.Context, projectID, appID string) (applicationRecord, error) {
	var response struct {
		App applicationRecord `json:"app"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/management/v1/projects/"+url.PathEscape(projectID)+"/apps/"+url.PathEscape(appID), nil, &response); err != nil {
		return applicationRecord{}, err
	}
	return response.App.normalized(), nil
}

func (c client) createAPIApplication(ctx context.Context, projectID, name string) (applicationRecord, error) {
	var response struct {
		AppID        string `json:"appId"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(projectID)+"/apps/api", map[string]any{
		"name": name, "authMethodType": "API_AUTH_METHOD_TYPE_BASIC",
	}, &response); err != nil {
		return applicationRecord{}, err
	}
	return applicationRecord{ID: response.AppID, Name: name, APIConfig: &apiApplicationConfig{
		ClientID: response.ClientID, ClientSecret: response.ClientSecret, AuthMethodType: "API_AUTH_METHOD_TYPE_BASIC",
	}}, nil
}

func (c client) updateAPIApplicationConfig(ctx context.Context, projectID, appID string) error {
	path := "/management/v1/projects/" + url.PathEscape(projectID) + "/apps/" + url.PathEscape(appID) + "/api_config"
	if err := c.doJSON(ctx, http.MethodPut, path, map[string]any{
		"authMethodType": "API_AUTH_METHOD_TYPE_BASIC",
	}, nil); err != nil {
		return fmt.Errorf("update local API application authentication: %w", err)
	}
	return nil
}

func (c client) regenerateAPIClientSecret(ctx context.Context, projectID, appID string) (string, error) {
	var response struct {
		ClientSecret string `json:"clientSecret"`
	}
	path := "/management/v1/projects/" + url.PathEscape(projectID) + "/apps/" + url.PathEscape(appID) + "/api_config/_generate_client_secret"
	if err := c.doJSON(ctx, http.MethodPost, path, map[string]any{}, &response); err != nil {
		return "", fmt.Errorf("regenerate local API client secret: %w", err)
	}
	if strings.TrimSpace(response.ClientSecret) == "" {
		return "", errors.New("regenerate local API client secret returned an empty secret")
	}
	return response.ClientSecret, nil
}

func (c client) regenerateOIDCClientSecret(ctx context.Context, projectID, appID string) (string, error) {
	var response struct {
		ClientSecret string `json:"clientSecret"`
	}
	path := "/management/v1/projects/" + url.PathEscape(projectID) + "/apps/" + url.PathEscape(appID) + "/oidc_config/_generate_client_secret"
	if err := c.doJSON(ctx, http.MethodPost, path, map[string]any{}, &response); err != nil {
		return "", fmt.Errorf("regenerate local OIDC client secret: %w", err)
	}
	if strings.TrimSpace(response.ClientSecret) == "" {
		return "", errors.New("regenerate local OIDC client secret returned an empty secret")
	}
	return response.ClientSecret, nil
}

func (c client) createOIDCApplication(ctx context.Context, projectID string, cfg LocalApplicationConfig) (applicationRecord, error) {
	var response struct {
		AppID        string `json:"appId"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(projectID)+"/apps/oidc", map[string]any{
		"name":                     cfg.OIDCName,
		"redirectUris":             cfg.RedirectURIs,
		"responseTypes":            []string{"OIDC_RESPONSE_TYPE_CODE"},
		"grantTypes":               []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"},
		"appType":                  "OIDC_APP_TYPE_WEB",
		"authMethodType":           "OIDC_AUTH_METHOD_TYPE_BASIC",
		"version":                  "OIDC_VERSION_1_0",
		"accessTokenType":          "OIDC_TOKEN_TYPE_BEARER",
		"accessTokenRoleAssertion": true,
		"idTokenRoleAssertion":     true,
		"devMode":                  true,
		"postLogoutRedirectUris":   cfg.PostLogoutRedirectURIs,
	}, &response); err != nil {
		return applicationRecord{}, err
	}
	return applicationRecord{ID: response.AppID, Name: cfg.OIDCName, OIDCConfig: &oidcApplicationConfig{
		ClientID: response.ClientID, ClientSecret: response.ClientSecret,
	}}, nil
}

func (c client) updateOIDCApplicationConfig(ctx context.Context, projectID, appID string, cfg LocalApplicationConfig) error {
	path := "/management/v1/projects/" + url.PathEscape(projectID) + "/apps/" + url.PathEscape(appID) + "/oidc_config"
	return c.doJSON(ctx, http.MethodPut, path, map[string]any{
		"redirectUris":             cfg.RedirectURIs,
		"responseTypes":            []string{"OIDC_RESPONSE_TYPE_CODE"},
		"grantTypes":               []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"},
		"appType":                  "OIDC_APP_TYPE_WEB",
		"authMethodType":           "OIDC_AUTH_METHOD_TYPE_BASIC",
		"accessTokenType":          "OIDC_TOKEN_TYPE_BEARER",
		"accessTokenRoleAssertion": true,
		"idTokenRoleAssertion":     true,
		"devMode":                  true,
		"postLogoutRedirectUris":   cfg.PostLogoutRedirectURIs,
	}, nil)
}

type userGrant struct {
	ID        string   `json:"id"`
	UserID    string   `json:"userId"`
	OrgID     string   `json:"orgId"`
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

type organizationRecord struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

func (c client) listOrganizations(ctx context.Context, query map[string]any) ([]organizationRecord, error) {
	var result []organizationRecord
	for offset := 0; ; {
		var response struct {
			Details struct {
				TotalResult zitadelprotojson.Uint64 `json:"totalResult"`
			} `json:"details"`
			Result []organizationRecord `json:"result"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/v2/organizations/_search", map[string]any{
			"query":         map[string]any{"offset": offset, "limit": acceptanceListPageSize, "asc": true},
			"sortingColumn": "ORGANIZATION_FIELD_NAME_NAME",
			"queries":       []map[string]any{query},
		}, &response); err != nil {
			return nil, err
		}
		result = append(result, response.Result...)
		if len(result) > acceptanceListLimit || response.Details.TotalResult > acceptanceListLimit {
			return nil, errors.New("acceptance organization query exceeded the safe candidate limit")
		}
		if uint64(len(result)) >= uint64(response.Details.TotalResult) {
			return result, nil
		}
		if len(response.Result) == 0 {
			return nil, errors.New("acceptance organization pagination made no progress")
		}
		offset += len(response.Result)
	}
}

const (
	acceptanceListPageSize = 100
	acceptanceListLimit    = 1000
	acceptanceReadAttempts = 31
	acceptanceReadInterval = 100 * time.Millisecond
)

func waitForAcceptanceReadBack(ctx context.Context, verify func() (bool, error)) (bool, error) {
	for attempt := 0; attempt < acceptanceReadAttempts; attempt++ {
		matched, err := verify()
		if err != nil || matched {
			return matched, err
		}
		if attempt == acceptanceReadAttempts-1 {
			break
		}
		timer := time.NewTimer(acceptanceReadInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, nil
}

func (c client) findOrganizationByID(ctx context.Context, organizationID string) (organizationRecord, bool, error) {
	records, err := c.listOrganizations(ctx, map[string]any{"idQuery": map[string]any{"id": organizationID}})
	if err != nil {
		return organizationRecord{}, false, err
	}
	matches := make([]organizationRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ID) == organizationID {
			matches = append(matches, record)
		}
	}
	if len(matches) > 1 {
		return organizationRecord{}, false, errors.New("multiple organizations matched the stable acceptance organization id")
	}
	if len(matches) == 0 {
		return organizationRecord{}, false, nil
	}
	return matches[0], true, nil
}

func (c client) findOrganizationByName(ctx context.Context, name string) (organizationRecord, bool, error) {
	records, err := c.listOrganizations(ctx, map[string]any{"nameQuery": map[string]any{"name": name, "method": "TEXT_QUERY_METHOD_EQUALS"}})
	if err != nil {
		return organizationRecord{}, false, err
	}
	matches := make([]organizationRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Name) == name {
			matches = append(matches, record)
		}
	}
	if len(matches) > 1 {
		return organizationRecord{}, false, errors.New("multiple organizations matched the acceptance organization name")
	}
	if len(matches) == 0 {
		return organizationRecord{}, false, nil
	}
	return matches[0], true, nil
}

func (c client) createOrganization(ctx context.Context, name string) (string, error) {
	var response struct {
		OrganizationID string `json:"organizationId"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v2/organizations", map[string]any{"name": name}, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.OrganizationID) == "" {
		return "", errors.New("ZITADEL create organization response did not include an id")
	}
	return strings.TrimSpace(response.OrganizationID), nil
}

func (c client) activateOrganization(ctx context.Context, organizationID string) error {
	return c.doJSON(ctx, http.MethodPost, "/v2/organizations/"+url.PathEscape(organizationID)+"/activate", nil, &struct{}{})
}

func (c client) ensureAcceptanceOrganization(ctx context.Context, preferredOrganizationID, name string) (string, error) {
	existing, found, err := c.findOrganizationByID(ctx, preferredOrganizationID)
	if err != nil {
		return "", fmt.Errorf("find acceptance organization by preferred id: %w", err)
	}
	if found && strings.TrimSpace(existing.Name) != name {
		return "", errors.New("preferred acceptance organization id is owned by a different organization")
	}
	if !found {
		existing, found, err = c.findOrganizationByName(ctx, name)
		if err != nil {
			return "", fmt.Errorf("check acceptance organization name ownership: %w", err)
		}
		if !found {
			createdID, createErr := c.createOrganization(ctx, name)
			if createErr != nil && !isProviderConflict(createErr) {
				return "", fmt.Errorf("create acceptance organization: %w", createErr)
			}
			matched, readErr := waitForAcceptanceReadBack(ctx, func() (bool, error) {
				if createErr == nil {
					existing, found, err = c.findOrganizationByID(ctx, createdID)
				} else {
					existing, found, err = c.findOrganizationByName(ctx, name)
				}
				return found && strings.TrimSpace(existing.Name) == name, err
			})
			if readErr != nil {
				return "", fmt.Errorf("read back created acceptance organization: %w", readErr)
			}
			if !matched {
				return "", errors.New("acceptance organization ownership read-back did not match")
			}
		}
	}
	organizationID := strings.TrimSpace(existing.ID)
	if organizationID == "" || strings.TrimSpace(existing.Name) != name {
		return "", errors.New("acceptance organization ownership read-back did not match")
	}
	switch existing.State {
	case "ORGANIZATION_STATE_ACTIVE":
		return organizationID, nil
	case "ORGANIZATION_STATE_INACTIVE", "ORGANIZATION_STATE_DEACTIVATED":
		if err := c.activateOrganization(ctx, organizationID); err != nil {
			return "", fmt.Errorf("activate acceptance organization: %w", err)
		}
		matched, readErr := waitForAcceptanceReadBack(ctx, func() (bool, error) {
			existing, found, err = c.findOrganizationByID(ctx, organizationID)
			if err != nil || !found || strings.TrimSpace(existing.Name) != name {
				return false, err
			}
			return existing.State == "ORGANIZATION_STATE_ACTIVE", nil
		})
		if readErr != nil {
			return "", fmt.Errorf("read back activated acceptance organization: %w", readErr)
		}
		if !matched {
			return "", errors.New("acceptance organization activation read-back did not match")
		}
		return organizationID, nil
	default:
		return "", errors.New("acceptance organization is not active")
	}
}

type projectGrantRecord struct {
	ProjectID             string   `json:"projectId"`
	GrantedOrganizationID string   `json:"grantedOrganizationId"`
	GrantedRoleKeys       []string `json:"grantedRoleKeys"`
	State                 string   `json:"state"`
}

func (c client) listProjectGrants(ctx context.Context, projectID, organizationID string) ([]projectGrantRecord, error) {
	var result []projectGrantRecord
	for offset := 0; ; {
		var response struct {
			Pagination struct {
				TotalResult zitadelprotojson.Uint64 `json:"totalResult"`
			} `json:"pagination"`
			ProjectGrants []projectGrantRecord `json:"projectGrants"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/zitadel.project.v2.ProjectService/ListProjectGrants", map[string]any{
			"pagination":    map[string]any{"offset": offset, "limit": acceptanceListPageSize, "asc": true},
			"sortingColumn": "PROJECT_GRANT_FIELD_NAME_CREATION_DATE",
			"filters": []map[string]any{
				{"inProjectIdsFilter": map[string]any{"ids": []string{projectID}}},
				{"grantedOrganizationIdFilter": map[string]any{"id": organizationID}},
			},
		}, &response); err != nil {
			return nil, err
		}
		for _, grant := range response.ProjectGrants {
			if grant.ProjectID == projectID && grant.GrantedOrganizationID == organizationID {
				result = append(result, grant)
			}
		}
		if len(result) > acceptanceListLimit || response.Pagination.TotalResult > acceptanceListLimit {
			return nil, errors.New("acceptance project grant query exceeded the safe candidate limit")
		}
		if uint64(len(result)) >= uint64(response.Pagination.TotalResult) {
			return result, nil
		}
		if len(response.ProjectGrants) == 0 {
			return nil, errors.New("acceptance project grant pagination made no progress")
		}
		offset += len(response.ProjectGrants)
	}
}

func (c client) createProjectGrant(ctx context.Context, projectID, organizationID string, roleKeys []string) error {
	return c.doJSON(ctx, http.MethodPost, "/zitadel.project.v2.ProjectService/CreateProjectGrant", map[string]any{
		"projectId": projectID, "grantedOrganizationId": organizationID, "roleKeys": roleKeys,
	}, &struct{}{})
}

func (c client) updateProjectGrant(ctx context.Context, projectID, organizationID string, roleKeys []string) error {
	return c.doJSON(ctx, http.MethodPost, "/zitadel.project.v2.ProjectService/UpdateProjectGrant", map[string]any{
		"projectId": projectID, "grantedOrganizationId": organizationID, "roleKeys": roleKeys,
	}, &struct{}{})
}

func (c client) activateProjectGrant(ctx context.Context, projectID, organizationID string) error {
	return c.doJSON(ctx, http.MethodPost, "/zitadel.project.v2.ProjectService/ActivateProjectGrant", map[string]any{
		"projectId": projectID, "grantedOrganizationId": organizationID,
	}, &struct{}{})
}

func (c client) ensureProjectGrant(ctx context.Context, projectID, organizationID string, roleKeys []string) error {
	grants, err := c.listProjectGrants(ctx, projectID, organizationID)
	if err != nil {
		return fmt.Errorf("list acceptance project grant: %w", err)
	}
	if len(grants) > 1 {
		return errors.New("multiple acceptance project grants matched the exact project and organization")
	}
	if len(grants) == 0 {
		if err := c.createProjectGrant(ctx, projectID, organizationID, roleKeys); err != nil && !isProviderConflict(err) {
			return fmt.Errorf("create acceptance project grant: %w", err)
		}
	} else {
		grant := grants[0]
		if !equalStringSets(grant.GrantedRoleKeys, roleKeys) {
			if err := c.updateProjectGrant(ctx, projectID, organizationID, roleKeys); err != nil {
				return fmt.Errorf("update acceptance project grant: %w", err)
			}
		}
		if grant.State == "PROJECT_GRANT_STATE_INACTIVE" || grant.State == "PROJECT_GRANT_STATE_DEACTIVATED" {
			if err := c.activateProjectGrant(ctx, projectID, organizationID); err != nil {
				return fmt.Errorf("activate acceptance project grant: %w", err)
			}
		} else if grant.State != "PROJECT_GRANT_STATE_ACTIVE" {
			return errors.New("acceptance project grant is not active")
		}
	}
	return c.verifyProjectGrant(ctx, projectID, organizationID, roleKeys)
}

func (c client) verifyProjectGrant(ctx context.Context, projectID, organizationID string, roleKeys []string) error {
	matched, err := waitForAcceptanceReadBack(ctx, func() (bool, error) {
		grants, err := c.listProjectGrants(ctx, projectID, organizationID)
		if err != nil {
			return false, err
		}
		if len(grants) > 1 {
			return false, errors.New("multiple acceptance project grants matched the exact project and organization")
		}
		return len(grants) == 1 && grants[0].State == "PROJECT_GRANT_STATE_ACTIVE" && equalStringSets(grants[0].GrantedRoleKeys, roleKeys), nil
	})
	if err != nil {
		return fmt.Errorf("read back acceptance project grant: %w", err)
	}
	if !matched {
		return errors.New("acceptance project grant read-back did not match active exact roles")
	}
	return nil
}

func findProjectGrant(grants []projectGrantRecord, projectID, organizationID string) (projectGrantRecord, bool) {
	for _, grant := range grants {
		if grant.ProjectID == projectID && grant.GrantedOrganizationID == organizationID {
			return grant, true
		}
	}
	return projectGrantRecord{}, false
}

type authorizationRecord struct {
	ID      string `json:"id"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	Organization struct {
		ID string `json:"id"`
	} `json:"organization"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Roles []struct {
		Key string `json:"key"`
	} `json:"roles"`
	RoleKeys []string `json:"-"`
	State    string   `json:"state"`
}

func (c client) listAuthorizations(ctx context.Context, userID, projectID, organizationID string) ([]authorizationRecord, error) {
	var result []authorizationRecord
	for offset := 0; ; {
		var response struct {
			Pagination struct {
				TotalResult zitadelprotojson.Uint64 `json:"totalResult"`
			} `json:"pagination"`
			Authorizations []authorizationRecord `json:"authorizations"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations", map[string]any{
			"pagination":    map[string]any{"offset": offset, "limit": acceptanceListPageSize, "asc": true},
			"sortingColumn": "AUTHORIZATION_FIELD_NAME_ID",
			"filters": []map[string]any{
				{"inUserIds": map[string]any{"ids": []string{userID}}},
				{"projectId": map[string]any{"id": projectID}},
				{"organizationId": map[string]any{"id": organizationID}},
			},
		}, &response); err != nil {
			return nil, err
		}
		for index := range response.Authorizations {
			for _, role := range response.Authorizations[index].Roles {
				response.Authorizations[index].RoleKeys = append(response.Authorizations[index].RoleKeys, role.Key)
			}
		}
		for _, authorization := range response.Authorizations {
			if authorization.User.ID == userID && authorization.Project.ID == projectID && authorization.Organization.ID == organizationID {
				result = append(result, authorization)
			}
		}
		if len(result) > acceptanceListLimit || response.Pagination.TotalResult > acceptanceListLimit {
			return nil, errors.New("acceptance authorization query exceeded the safe candidate limit")
		}
		if uint64(len(result)) >= uint64(response.Pagination.TotalResult) {
			return result, nil
		}
		if len(response.Authorizations) == 0 {
			return nil, errors.New("acceptance authorization pagination made no progress")
		}
		offset += len(response.Authorizations)
	}
}

func findAuthorization(authorizations []authorizationRecord, userID, projectID, organizationID string) (authorizationRecord, bool) {
	for _, authorization := range authorizations {
		if authorization.User.ID == userID && authorization.Project.ID == projectID && authorization.Organization.ID == organizationID {
			return authorization, true
		}
	}
	return authorizationRecord{}, false
}

func (c client) activateAuthorization(ctx context.Context, authorizationID string) error {
	return c.doJSON(ctx, http.MethodPost, "/zitadel.authorization.v2.AuthorizationService/ActivateAuthorization", map[string]any{"id": authorizationID}, &struct{}{})
}

func (c client) ensureAuthorization(ctx context.Context, userID, projectID, organizationID string, roleKeys []string) error {
	authorizations, err := c.listAuthorizations(ctx, userID, projectID, organizationID)
	if err != nil {
		return fmt.Errorf("list acceptance role assignment: %w", err)
	}
	if len(authorizations) > 1 {
		return errors.New("multiple acceptance role assignments matched the exact user, project, and organization")
	}
	if len(authorizations) == 0 {
		identity := authidentity.AuthenticatedIdentity{TenantID: organizationID, UserID: userID}
		if err := c.createAuthorization(ctx, projectID, identity, roleKeys); err != nil && !isProviderConflict(err) {
			return fmt.Errorf("create acceptance role assignment: %w", err)
		}
	} else {
		authorization := authorizations[0]
		if !equalStringSets(authorization.RoleKeys, roleKeys) {
			if err := c.updateAuthorization(ctx, authorization.ID, roleKeys); err != nil {
				return fmt.Errorf("update acceptance role assignment: %w", err)
			}
		}
		if authorization.State == "STATE_INACTIVE" {
			if err := c.activateAuthorization(ctx, authorization.ID); err != nil {
				return fmt.Errorf("activate acceptance role assignment: %w", err)
			}
		} else if authorization.State != "STATE_ACTIVE" {
			return errors.New("acceptance role assignment is not active")
		}
	}
	return c.verifyAuthorization(ctx, userID, projectID, organizationID, roleKeys)
}

func (c client) verifyAuthorization(ctx context.Context, userID, projectID, organizationID string, roleKeys []string) error {
	matched, err := waitForAcceptanceReadBack(ctx, func() (bool, error) {
		authorizations, err := c.listAuthorizations(ctx, userID, projectID, organizationID)
		if err != nil {
			return false, err
		}
		if len(authorizations) > 1 {
			return false, errors.New("multiple acceptance role assignments matched the exact user, project, and organization")
		}
		return len(authorizations) == 1 && authorizations[0].State == "STATE_ACTIVE" && equalStringSets(authorizations[0].RoleKeys, roleKeys), nil
	})
	if err != nil {
		return fmt.Errorf("read back acceptance role assignment: %w", err)
	}
	if !matched {
		return errors.New("acceptance role assignment read-back did not match active exact roles")
	}
	return nil
}

func (c client) verifyAcceptanceState(ctx context.Context, userID, projectID, organizationID, name string, roleKeys []string) error {
	organization, found, err := c.findOrganizationByID(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("final acceptance organization read-back: %w", err)
	}
	if !found || organization.State != "ORGANIZATION_STATE_ACTIVE" || strings.TrimSpace(organization.Name) != name {
		return errors.New("final acceptance organization state did not match")
	}
	if err := c.verifyProjectGrant(ctx, projectID, organizationID, roleKeys); err != nil {
		return err
	}
	return c.verifyAuthorization(ctx, userID, projectID, organizationID, roleKeys)
}

func (c client) findUserGrant(ctx context.Context, projectID string, identity authidentity.AuthenticatedIdentity) (userGrant, bool, error) {
	var response struct {
		Result []userGrant `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/users/grants/_search", map[string]any{
		"queries": []map[string]any{{"userIdQuery": map[string]any{"userId": identity.UserID}}},
	}, &response); err != nil {
		return userGrant{}, false, err
	}
	for _, grant := range response.Result {
		if grant.UserID == identity.UserID && grant.OrgID == identity.TenantID && grant.ProjectID == projectID {
			return grant, true, nil
		}
	}
	return userGrant{}, false, nil
}

func (c client) createAuthorization(ctx context.Context, projectID string, identity authidentity.AuthenticatedIdentity, roles []string) error {
	var response struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization", map[string]any{
		"userId": identity.UserID, "projectId": projectID, "organizationId": identity.TenantID, "roleKeys": roles,
	}, &response); err != nil {
		return err
	}
	if response.ID == "" {
		return errors.New("ZITADEL role assignment did not return an authorization id")
	}
	return nil
}

func (c client) updateAuthorization(ctx context.Context, authorizationID string, roles []string) error {
	return c.doJSON(ctx, http.MethodPost, "/zitadel.authorization.v2.AuthorizationService/UpdateAuthorization", map[string]any{
		"id": authorizationID, "roleKeys": roles,
	}, &struct{}{})
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalStringSets(left, right []string) bool {
	left = normalizeStrings(left)
	right = normalizeStrings(right)
	if len(left) != len(right) {
		return false
	}
	for _, value := range left {
		if !containsString(right, value) {
			return false
		}
	}
	return true
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

func (c client) createProject(ctx context.Context, name string, hasProjectCheck *bool) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	projectHasProjectCheck := true
	if hasProjectCheck != nil {
		projectHasProjectCheck = *hasProjectCheck
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects", map[string]any{
		"name":                 name,
		"projectRoleAssertion": true,
		"projectRoleCheck":     true,
		"hasProjectCheck":      projectHasProjectCheck,
	}, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("ZITADEL create project response did not include an id")
	}
	return response.ID, nil
}

func (c client) updateProject(ctx context.Context, projectID, name string, hasProjectCheck bool) error {
	var response map[string]any
	return c.doJSON(ctx, http.MethodPut, "/management/v1/projects/"+url.PathEscape(projectID), map[string]any{
		"name":                 name,
		"projectRoleAssertion": true,
		"projectRoleCheck":     true,
		"hasProjectCheck":      hasProjectCheck,
	}, &response)
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
	if strings.HasPrefix(path, "/zitadel.authorization.v2.AuthorizationService/") ||
		strings.HasPrefix(path, "/zitadel.project.v2.ProjectService/") {
		request.Header.Set("Connect-Protocol-Version", "1")
	}
	if c.orgID != "" {
		request.Header.Set("x-zitadel-orgid", c.orgID)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &providerHTTPError{StatusCode: response.StatusCode, Method: method, Path: path, Status: response.Status}
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(target)
}

type providerHTTPError struct {
	StatusCode int
	Method     string
	Path       string
	Status     string
}

func (e *providerHTTPError) Error() string {
	return fmt.Sprintf("ZITADEL %s %s failed: %s", e.Method, e.Path, e.Status)
}

func isProviderConflict(err error) bool {
	var providerErr *providerHTTPError
	return errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusConflict
}

func roleKeys(roles []ProjectRole) []string {
	keys := make([]string, 0, len(roles))
	for _, role := range roles {
		keys = append(keys, role.Key)
	}
	return keys
}
