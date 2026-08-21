package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"task-processor/internal/localagent"
	"task-processor/internal/product/sourcing"
)

type Client struct {
	BaseURL     string
	AccessToken string
	HTTPClient  *http.Client
}

func New(baseURL, accessToken string, httpClient *http.Client) (*Client, error) {
	base, err := validateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("access token is required")
	}
	return &Client{BaseURL: strings.TrimRight(base.String(), "/"), AccessToken: strings.TrimSpace(accessToken), HTTPClient: safeHTTPClient(httpClient)}, nil
}

func (c *Client) CreateJob(ctx context.Context, rawURL string) (localagent.Job, error) {
	var response jobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs", map[string]string{"url": rawURL}, http.StatusCreated, &response)
	return response.toJob(), err
}

func (c *Client) Claim(ctx context.Context) (*localagent.Claim, error) {
	var response claimResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/claim", nil, http.StatusOK, &response)
	if errors.Is(err, errNoJob) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &localagent.Claim{Job: response.toJob(), ExecutionToken: response.ExecutionToken}, nil
}

func (c *Client) SubmitSuccess(ctx context.Context, jobID, token string, snapshot *sourcing.Alibaba1688ProductSnapshot) (localagent.Job, error) {
	var response jobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/"+url.PathEscape(jobID)+"/result", map[string]any{
		"execution_token":  token,
		"product_snapshot": snapshot,
	}, http.StatusOK, &response)
	return response.toJob(), err
}

func (c *Client) SubmitFailure(ctx context.Context, jobID, token string, failure localagent.Failure) (localagent.Job, error) {
	var response jobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/"+url.PathEscape(jobID)+"/result", map[string]any{
		"execution_token": token,
		"failure":         failure,
	}, http.StatusOK, &response)
	return response.toJob(), err
}

var errNoJob = errors.New("no local-agent job")

type jobResponse struct {
	JobID          string                   `json:"job_id"`
	TenantID       string                   `json:"tenant_id"`
	URL            string                   `json:"url"`
	State          localagent.JobState      `json:"state"`
	ExpiresAt      time.Time                `json:"expires_at"`
	LeaseExpiresAt time.Time                `json:"lease_expires_at"`
	Envelope       *sourcing.SourceEnvelope `json:"envelope"`
	Failure        *localagent.Failure      `json:"failure"`
}

type claimResponse struct {
	JobID          string    `json:"job_id"`
	ExecutionToken string    `json:"execution_token"`
	URL            string    `json:"url"`
	ExpiresAt      time.Time `json:"expires_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

func (r jobResponse) toJob() localagent.Job {
	return localagent.Job{ID: r.JobID, TenantID: r.TenantID, URL: r.URL, State: r.State, ExpiresAt: r.ExpiresAt, LeaseExpiresAt: r.LeaseExpiresAt, Envelope: r.Envelope, Failure: r.Failure}
}

func (r claimResponse) toJob() localagent.Job {
	return localagent.Job{ID: r.JobID, URL: r.URL, ExpiresAt: r.ExpiresAt, LeaseExpiresAt: r.LeaseExpiresAt, State: localagent.JobClaimed}
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, expected int, target any) error {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.AccessToken) == "" {
		return errors.New("local-agent client is not configured")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return errors.New("request could not be encoded")
		}
		body = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return errors.New("local-agent request could not be created")
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return errors.New("local-agent request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent && expected == http.StatusOK {
		return errNoJob
	}
	if resp.StatusCode != expected {
		return fmt.Errorf("local-agent request returned status %d", resp.StatusCode)
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(target); err != nil {
		return errors.New("local-agent response was invalid")
	}
	return nil
}

func validateBaseURL(raw string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("api base URL must be an absolute HTTPS URI")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && isLoopback(base.Hostname())) {
		return nil, errors.New("api base URL must use HTTPS unless it is a literal loopback test endpoint")
	}
	return base, nil
}

func safeHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &clone
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
