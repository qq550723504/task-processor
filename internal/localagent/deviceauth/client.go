package deviceauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	IssuerURL  string
	ClientID   string
	ProjectID  string
	Scopes     string
	Timeout    time.Duration
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

type Presenter interface {
	Show(verificationURI, userCode string) error
}

type discoveryDocument struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

type deviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

func Authorize(ctx context.Context, cfg Config, presenter Presenter) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return "", errors.New("client ID is required")
	}
	issuer, err := validateEndpoint(cfg.IssuerURL, "issuer")
	if err != nil {
		return "", err
	}
	scopes, err := oauthScopes(cfg.Scopes, cfg.ProjectID)
	if err != nil {
		return "", err
	}
	client := safeHTTPClient(cfg.HTTPClient)
	deadline := time.Now().Add(cfg.Timeout)
	if cfg.Timeout <= 0 {
		deadline = time.Now().Add(5 * time.Minute)
	}
	requestContext, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	discoveryURI := issuer.ResolveReference(&url.URL{Path: strings.TrimRight(issuer.Path, "/") + "/.well-known/openid-configuration"})
	var discovery discoveryDocument
	if err := doJSON(requestContext, client, http.MethodGet, discoveryURI, nil, "discovery", &discovery); err != nil {
		return "", err
	}
	deviceURI, err := sameOriginEndpoint(issuer, discovery.DeviceAuthorizationEndpoint, "device authorization endpoint")
	if err != nil {
		return "", err
	}
	tokenURI, err := sameOriginEndpoint(issuer, discovery.TokenEndpoint, "token endpoint")
	if err != nil {
		return "", err
	}
	form := url.Values{"client_id": {strings.TrimSpace(cfg.ClientID)}, "scope": {scopes}}
	var device deviceResponse
	if err := doJSON(requestContext, client, http.MethodPost, deviceURI, form, "device authorization", &device); err != nil {
		return "", err
	}
	if strings.TrimSpace(device.DeviceCode) == "" || strings.TrimSpace(device.UserCode) == "" || device.ExpiresIn <= 0 {
		return "", errors.New("device authorization response is incomplete")
	}
	verificationURI, err := sameOriginEndpoint(issuer, device.VerificationURI, "verification URI")
	if err != nil {
		return "", err
	}
	if presenter != nil {
		if err := presenter.Show(verificationURI.String(), device.UserCode); err != nil {
			return "", err
		}
	}
	interval := time.Duration(device.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deviceDeadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	if deviceDeadline.Before(deadline) {
		deadline = deviceDeadline
	}
	pollContext, pollCancel := context.WithDeadline(ctx, deadline)
	defer pollCancel()
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	for {
		if err := sleep(pollContext, interval); err != nil {
			return "", errors.New("device authorization timed out")
		}
		var token tokenResponse
		if err := doJSON(pollContext, client, http.MethodPost, tokenURI, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {device.DeviceCode},
			"client_id":   {strings.TrimSpace(cfg.ClientID)},
		}, "token exchange", &token); err != nil {
			return "", err
		}
		if strings.TrimSpace(token.RefreshToken) != "" {
			return "", errors.New("device authorization returned a refresh token")
		}
		if strings.TrimSpace(token.AccessToken) != "" {
			return strings.TrimSpace(token.AccessToken), nil
		}
		switch strings.TrimSpace(token.Error) {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied":
			return "", errors.New("device authorization was denied")
		case "expired_token":
			return "", errors.New("device authorization expired")
		default:
			return "", errors.New("device authorization token exchange failed")
		}
	}
}

func oauthScopes(raw, projectID string) (string, error) {
	scopes := strings.TrimSpace(raw)
	for _, scope := range strings.Fields(scopes) {
		if scope == "offline_access" {
			return "", errors.New("offline_access is not allowed for device authorization")
		}
	}
	if scopes != "" {
		return scopes, nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || len(strings.Fields(projectID)) != 1 {
		return "", errors.New("project ID is required for device authorization")
	}
	return strings.Join([]string{
		"openid", "profile", "email", "urn:zitadel:iam:user:resourceowner",
		"urn:zitadel:iam:org:project:id:" + projectID + ":aud",
		"urn:zitadel:iam:org:project:role:listingkit_viewer",
		"urn:zitadel:iam:org:project:role:listingkit_operator",
		"urn:zitadel:iam:org:project:role:listingkit_admin",
		"urn:zitadel:iam:org:project:role:platform_admin",
		"urn:zitadel:iam:org:project:role:admin",
	}, " "), nil
}

func validateEndpoint(raw, name string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must be an absolute HTTPS URI", name)
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopback(parsed.Hostname())) {
		return nil, fmt.Errorf("%s must use HTTPS unless it is a literal loopback test endpoint", name)
	}
	return parsed, nil
}

func sameOriginEndpoint(issuer *url.URL, raw, name string) (*url.URL, error) {
	candidate, err := validateEndpoint(raw, name)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(candidate.Scheme, issuer.Scheme) ||
		!strings.EqualFold(candidate.Hostname(), issuer.Hostname()) ||
		effectivePort(candidate) != effectivePort(issuer) {
		return nil, fmt.Errorf("%s must use the same origin as the issuer", name)
	}
	return candidate, nil
}

func effectivePort(endpoint *url.URL) int {
	if port := endpoint.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err == nil {
			return value
		}
	}
	if strings.EqualFold(endpoint.Scheme, "http") {
		return 80
	}
	if strings.EqualFold(endpoint.Scheme, "https") {
		return 443
	}
	return 0
}

func doJSON(ctx context.Context, client *http.Client, method string, endpoint *url.URL, form url.Values, operation string, target any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("%s request failed", operation)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed", operation)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s response was invalid", operation)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if operation == "token exchange" {
			if token, ok := target.(*tokenResponse); ok && strings.TrimSpace(token.AccessToken) == "" && strings.TrimSpace(token.RefreshToken) == "" {
				return nil
			}
		}
		return fmt.Errorf("%s request failed with status %d", operation, resp.StatusCode)
	}
	return nil
}

func safeHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &clone
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
