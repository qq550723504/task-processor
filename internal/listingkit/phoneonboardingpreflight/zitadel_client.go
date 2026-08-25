// Package phoneonboardingpreflight contains the narrow ZITADEL Login V2
// contract probe used by the native phone onboarding feasibility gate.
package phoneonboardingpreflight

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPClientTimeout = 10 * time.Second
	maxProviderResponseBytes = 1 << 20
	organizationManagerToken = "organization manager"
	loginClientSessionToken  = "login client"
)

// Client performs only the ZITADEL operations required by the preflight.
type Client interface {
	CreateOrganization(context.Context, string) (string, error)
	CreateTechnicalUser(context.Context, TechnicalUserInput) (string, error)
	AddOTPSMS(context.Context, string) error
	CreateSMSChallenge(context.Context, string, time.Duration) (SessionMaterial, error)
	VerifySMS(context.Context, string, string) (string, error)
	GetSession(context.Context, string, string) (SessionProof, error)
	DeleteSession(context.Context, string) error
}

type TechnicalUserInput struct {
	OrganizationID string
	Username       string
	TechnicalEmail string
	Phone          string
}

type SessionMaterial struct{ ID, Token string }

type SessionProof struct {
	UserID           string
	OrganizationID   string
	UserVerifiedAt   time.Time
	OTPSMSVerifiedAt time.Time
}

type ClientConfig struct {
	IssuerURL         string
	ProvisioningToken string
	SessionToken      string
	HTTPClient        *http.Client
}

type zitadelClient struct {
	baseURL           string
	provisioningToken string
	sessionToken      string
	http              *http.Client
}

// NewClient constructs a version-pinned Login V2 REST client. Provisioning and
// session operations deliberately use different short-lived credentials.
func NewClient(cfg ClientConfig) (Client, error) {
	baseURL, err := validateIssuerURL(cfg.IssuerURL)
	if err != nil {
		return nil, err
	}
	provisioningToken := strings.TrimSpace(cfg.ProvisioningToken)
	if provisioningToken == "" {
		return nil, errors.New("ZITADEL organization manager token is required")
	}
	sessionToken := strings.TrimSpace(cfg.SessionToken)
	if sessionToken == "" {
		return nil, errors.New("ZITADEL login client token is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPClientTimeout}
	}
	return &zitadelClient{
		baseURL: baseURL, provisioningToken: provisioningToken, sessionToken: sessionToken, http: httpClient,
	}, nil
}

func (c *zitadelClient) CreateOrganization(ctx context.Context, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("create ZITADEL organization: invalid input")
	}
	var response struct {
		OrganizationID string `json:"organizationId"`
	}
	if err := c.doJSON(ctx, organizationManagerToken, http.MethodPost, "/v2/organizations", nil, struct {
		Name string `json:"name"`
	}{Name: name}, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.OrganizationID) == "" {
		return "", errors.New("ZITADEL organization creation returned no id")
	}
	return response.OrganizationID, nil
}

func (c *zitadelClient) CreateTechnicalUser(ctx context.Context, in TechnicalUserInput) (string, error) {
	if strings.TrimSpace(in.OrganizationID) == "" || strings.TrimSpace(in.Username) == "" ||
		strings.TrimSpace(in.TechnicalEmail) == "" || strings.TrimSpace(in.Phone) == "" {
		return "", errors.New("create ZITADEL technical user: invalid input")
	}
	body := createUserRequest{
		OrganizationID: in.OrganizationID,
		Username:       in.Username,
		Human: createHuman{
			Profile: humanProfile{GivenName: "Phone", FamilyName: "Preflight", DisplayName: "Phone Preflight"},
			Email:   verifiedEmail{Email: in.TechnicalEmail, IsVerified: true},
			Phone:   verifiedPhone{Phone: in.Phone, IsVerified: true},
		},
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, organizationManagerToken, http.MethodPost, "/v2/users/new", nil, body, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", errors.New("ZITADEL user creation returned no id")
	}
	return response.ID, nil
}

func (c *zitadelClient) AddOTPSMS(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("add ZITADEL OTP SMS: invalid input")
	}
	return c.doJSON(ctx, organizationManagerToken, http.MethodPost, "/v2/users/"+url.PathEscape(userID)+"/otp_sms", nil, struct{}{}, nil)
}

func (c *zitadelClient) CreateSMSChallenge(ctx context.Context, userID string, lifetime time.Duration) (SessionMaterial, error) {
	if strings.TrimSpace(userID) == "" || lifetime <= 0 {
		return SessionMaterial{}, errors.New("create ZITADEL SMS session: invalid input")
	}
	body := createSessionRequest{
		Checks:     checks{User: &checkUser{UserID: userID}},
		Challenges: requestChallenges{OTPSMS: &otpSMSChallenge{ReturnCode: false}},
		Lifetime:   formatProtoDuration(lifetime),
	}
	var response struct {
		SessionID    string `json:"sessionId"`
		SessionToken string `json:"sessionToken"`
	}
	if err := c.doJSON(ctx, loginClientSessionToken, http.MethodPost, "/v2/sessions", nil, body, &response); err != nil {
		return SessionMaterial{}, err
	}
	if strings.TrimSpace(response.SessionID) == "" || strings.TrimSpace(response.SessionToken) == "" {
		return SessionMaterial{}, errors.New("ZITADEL session creation returned incomplete material")
	}
	return SessionMaterial{ID: response.SessionID, Token: response.SessionToken}, nil
}

func (c *zitadelClient) VerifySMS(ctx context.Context, sessionID, code string) (string, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(code) == "" {
		return "", errors.New("verify ZITADEL SMS session: invalid input")
	}
	var response struct {
		SessionToken string `json:"sessionToken"`
	}
	body := struct {
		Checks checks `json:"checks"`
	}{Checks: checks{OTPSMS: &checkOTP{Code: code}}}
	if err := c.doJSON(ctx, loginClientSessionToken, http.MethodPatch, "/v2/sessions/"+url.PathEscape(sessionID), nil, body, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.SessionToken) == "" {
		return "", errors.New("ZITADEL SMS session verification returned no replacement token")
	}
	return response.SessionToken, nil
}

func (c *zitadelClient) GetSession(ctx context.Context, sessionID, sessionToken string) (SessionProof, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(sessionToken) == "" {
		return SessionProof{}, errors.New("get ZITADEL session: invalid input")
	}
	query := url.Values{"sessionToken": []string{sessionToken}}
	var response struct {
		Session struct {
			Factors struct {
				User *struct {
					ID             string `json:"id"`
					OrganizationID string `json:"organizationId"`
					VerifiedAt     string `json:"verifiedAt"`
				} `json:"user"`
				OTPSMS *struct {
					VerifiedAt string `json:"verifiedAt"`
				} `json:"otpSms"`
			} `json:"factors"`
		} `json:"session"`
	}
	if err := c.doJSON(ctx, loginClientSessionToken, http.MethodGet, "/v2/sessions/"+url.PathEscape(sessionID), query, nil, &response); err != nil {
		return SessionProof{}, err
	}
	if response.Session.Factors.User == nil || response.Session.Factors.OTPSMS == nil {
		return SessionProof{}, errors.New("ZITADEL session does not contain verified user and OTP SMS factors")
	}
	userVerifiedAt, err := time.Parse(time.RFC3339Nano, response.Session.Factors.User.VerifiedAt)
	if err != nil || strings.TrimSpace(response.Session.Factors.User.ID) == "" || strings.TrimSpace(response.Session.Factors.User.OrganizationID) == "" {
		return SessionProof{}, errors.New("ZITADEL session contains invalid verified user factor")
	}
	otpSMSVerifiedAt, err := time.Parse(time.RFC3339Nano, response.Session.Factors.OTPSMS.VerifiedAt)
	if err != nil {
		return SessionProof{}, errors.New("ZITADEL session contains invalid verified OTP SMS factor")
	}
	return SessionProof{
		UserID: response.Session.Factors.User.ID, OrganizationID: response.Session.Factors.User.OrganizationID,
		UserVerifiedAt: userVerifiedAt, OTPSMSVerifiedAt: otpSMSVerifiedAt,
	}, nil
}

func (c *zitadelClient) DeleteSession(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("delete ZITADEL session: invalid input")
	}
	return c.doJSON(ctx, loginClientSessionToken, http.MethodDelete, "/v2/sessions/"+url.PathEscape(sessionID), nil, nil, nil)
}

func (c *zitadelClient) doJSON(ctx context.Context, credential, method, path string, query url.Values, body, output any) error {
	operation := operationFor(method, path)
	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: encode request failed", operation)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return fmt.Errorf("%s: create request failed", operation)
	}
	request.Header.Set("Authorization", "Bearer "+c.tokenFor(credential))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%s: %w", operation, ctxErr)
		}
		return fmt.Errorf("%s: request failed", operation)
	}
	defer response.Body.Close()
	responseBody, err := readProviderResponse(response.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s: ZITADEL returned HTTP status %d", operation, response.StatusCode)
	}
	if output == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("%s: decode response failed", operation)
	}
	return nil
}

func (c *zitadelClient) tokenFor(credential string) string {
	if credential == organizationManagerToken {
		return c.provisioningToken
	}
	return c.sessionToken
}

func operationFor(method, path string) string {
	switch {
	case method == http.MethodPost && path == "/v2/organizations":
		return "create ZITADEL organization"
	case method == http.MethodPost && path == "/v2/users/new":
		return "create ZITADEL technical user"
	case method == http.MethodPost && strings.HasSuffix(path, "/otp_sms"):
		return "add ZITADEL OTP SMS"
	case method == http.MethodPost && path == "/v2/sessions":
		return "create ZITADEL SMS session"
	case method == http.MethodPatch:
		return "verify ZITADEL SMS session"
	case method == http.MethodGet:
		return "get ZITADEL session"
	case method == http.MethodDelete:
		return "delete ZITADEL session"
	default:
		return "call ZITADEL"
	}
}

func readProviderResponse(body io.Reader) ([]byte, error) {
	response, err := io.ReadAll(io.LimitReader(body, maxProviderResponseBytes+1))
	if err != nil {
		return nil, errors.New("read provider response failed")
	}
	if len(response) > maxProviderResponseBytes {
		return nil, errors.New("provider response exceeded size limit")
	}
	return response, nil
}

func validateIssuerURL(rawIssuerURL string) (string, error) {
	issuerURL := strings.TrimRight(strings.TrimSpace(rawIssuerURL), "/")
	if issuerURL == "" {
		return "", errors.New("ZITADEL issuer URL is required")
	}
	parsed, err := url.ParseRequestURI(issuerURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("ZITADEL issuer URL is invalid")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", errors.New("ZITADEL issuer URL must use HTTPS outside loopback")
	}
	return issuerURL, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func formatProtoDuration(duration time.Duration) string {
	seconds := duration / time.Second
	nanoseconds := duration % time.Second
	if nanoseconds == 0 {
		return strconv.FormatInt(int64(seconds), 10) + "s"
	}
	fraction := strings.TrimRight(fmt.Sprintf("%09d", nanoseconds), "0")
	return strconv.FormatInt(int64(seconds), 10) + "." + fraction + "s"
}

type createUserRequest struct {
	OrganizationID string      `json:"organizationId"`
	Username       string      `json:"username"`
	Human          createHuman `json:"human"`
}

type createHuman struct {
	Profile humanProfile  `json:"profile"`
	Email   verifiedEmail `json:"email"`
	Phone   verifiedPhone `json:"phone"`
}

type humanProfile struct {
	GivenName   string `json:"givenName"`
	FamilyName  string `json:"familyName"`
	DisplayName string `json:"displayName"`
}

type verifiedEmail struct {
	Email      string `json:"email"`
	IsVerified bool   `json:"isVerified"`
}

type verifiedPhone struct {
	Phone      string `json:"phone"`
	IsVerified bool   `json:"isVerified"`
}

type createSessionRequest struct {
	Checks     checks            `json:"checks"`
	Challenges requestChallenges `json:"challenges"`
	Lifetime   string            `json:"lifetime"`
}

type checks struct {
	User   *checkUser `json:"user,omitempty"`
	OTPSMS *checkOTP  `json:"otpSms,omitempty"`
}

type checkUser struct {
	UserID string `json:"userId"`
}

type checkOTP struct {
	Code string `json:"code"`
}

type requestChallenges struct {
	OTPSMS *otpSMSChallenge `json:"otpSms,omitempty"`
}

type otpSMSChallenge struct {
	ReturnCode bool `json:"returnCode"`
}
