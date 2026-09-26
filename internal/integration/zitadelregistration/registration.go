// Package zitadelregistration implements the pinned v4.17.1 create-only user API.
package zitadelregistration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/referral"
	"time"
)

type Config struct {
	Origin, LoginOrigin, Organization string
	Token                             func(context.Context) (string, error)
	HTTPClient                        *http.Client
}
type Client struct {
	config          Config
	http            *http.Client
	verificationURL string
}

const proofKey = "referral-registration-proof"
const verificationPath = "/ui/v2/login/verify?code={{.Code}}&userId={{.UserID}}&organization={{.OrgID}}"

func httpsOrigin(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.ContainsAny(raw, "?#") || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

func New(config Config) (*Client, error) {
	origin, validAPI := httpsOrigin(config.Origin)
	login, validLogin := httpsOrigin(config.LoginOrigin)
	if !validAPI || !validLogin || len(login+verificationPath) > 200 || config.Organization == "" || config.Token == nil {
		return nil, referral.ErrInvalid
	}
	config.Origin, config.LoginOrigin = origin, login
	client := http.Client{}
	if config.HTTPClient != nil {
		client = *config.HTTPClient
	}
	client.Timeout = 5 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{config: config, http: &client, verificationURL: login + verificationPath}, nil
}

func (c *Client) request(ctx context.Context, method, path string, body any, out any) error {
	if ctx == nil {
		return referral.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, err := c.config.Token(ctx)
	if err != nil || strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return referral.ErrUnavailable
	}
	data, err := json.Marshal(body)
	if err != nil || len(data) > 4096 {
		return referral.ErrInvalid
	}
	req, err := http.NewRequestWithContext(ctx, method, c.config.Origin+path, bytes.NewReader(data))
	if err != nil {
		return referral.ErrInvalid
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return referral.ErrUnknown
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 {
		return referral.ErrUnknown
	}
	if response.StatusCode == http.StatusNotFound && method == http.MethodGet {
		return referral.ErrMissing
	}
	if response.StatusCode == http.StatusConflict {
		return referral.ErrConflict
	}
	if method == http.MethodPost && path == "/v2/users/human" && response.StatusCode == http.StatusBadRequest && fixedIDAlreadyExists(data) {
		return referral.ErrConflict
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || json.Unmarshal(data, out) != nil {
		return referral.ErrUnknown
	}
	return nil
}

// The pinned AddUserHuman returns FailedPrecondition for an existing fixed ID.
// Match its typed ErrorDetail identity, never translated text or code 9 alone.
// https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/internal/command/user_v2_human.go
func fixedIDAlreadyExists(data []byte) bool {
	var status struct {
		Code    int
		Details []struct {
			Type string `json:"@type"`
			ID   string `json:"id"`
		}
	}
	if json.Unmarshal(data, &status) != nil || status.Code != 9 || len(status.Details) != 1 {
		return false
	}
	detail := status.Details[0]
	return detail.Type == "type.googleapis.com/zitadel.v1.ErrorDetail" && detail.ID == "COMMAND-7yiox1isql"
}

func (c *Client) Create(ctx context.Context, in app.Creation) error {
	if in.Organization != c.config.Organization || in.Subject == "" || len(in.Subject) > 200 || in.Email == "" || len(in.Email) > 200 || in.GivenName == "" || in.FamilyName == "" || len(in.GivenName) > 120 || len(in.FamilyName) > 120 || in.Proof == "" {
		return referral.ErrInvalid
	}
	body := map[string]any{
		"userId":       in.Subject,
		"organization": map[string]string{"orgId": in.Organization},
		"profile":      map[string]string{"givenName": in.GivenName, "familyName": in.FamilyName},
		"email":        map[string]any{"email": in.Email, "sendCode": map[string]string{"urlTemplate": c.verificationURL}},
		"metadata":     []map[string]string{{"key": proofKey, "value": base64.StdEncoding.EncodeToString([]byte(in.Proof))}},
	}
	var response struct {
		UserID string `json:"userId"`
	}
	if err := c.request(ctx, http.MethodPost, "/v2/users/human", body, &response); err != nil {
		return err
	}
	if response.UserID != in.Subject {
		return referral.ErrUnknown
	}
	return nil
}
