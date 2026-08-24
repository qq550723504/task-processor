package userdirectory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	listUsersPageSize        = 100
	defaultHTTPClientTimeout = 15 * time.Second
)

// User is the non-PII subset of a directory user needed by the preflight.
type User struct {
	Subject  string
	TenantID string
}

// Directory lists canonical user subjects within a tenant.
type Directory interface {
	ListByTenant(ctx context.Context, tenantID string) ([]User, error)
}

type ClientConfig struct {
	IssuerURL  string
	Token      string
	HTTPClient *http.Client
}

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient constructs a read-only ZITADEL user-directory client.
func NewClient(cfg ClientConfig) (Directory, error) {
	issuerURL := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	if issuerURL == "" {
		return nil, errors.New("ZITADEL issuer URL is required for the user directory")
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("ZITADEL read-only user directory token is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPClientTimeout}
	}
	return &client{baseURL: issuerURL, token: token, http: httpClient}, nil
}

func (c *client) ListByTenant(ctx context.Context, tenantID string) ([]User, error) {
	var (
		offset uint64
		users  []User
	)
	for {
		page, total, err := c.listPage(ctx, tenantID, offset)
		if err != nil {
			return nil, err
		}
		users = append(users, page...)
		offset += uint64(len(page))
		if offset >= total {
			return users, nil
		}
		if len(page) == 0 {
			return nil, errors.New("list ZITADEL users: response pagination made no progress")
		}
	}
}

func (c *client) listPage(ctx context.Context, tenantID string, offset uint64) ([]User, uint64, error) {
	payload := struct {
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
	}{}
	payload.Query.Offset = strconv.FormatUint(offset, 10)
	payload.Query.Limit = listUsersPageSize
	payload.Query.Asc = true
	payload.SortingColumn = "USER_FIELD_NAME_CREATION_DATE"
	payload.Queries = make([]struct {
		OrganizationIDQuery struct {
			OrganizationID string `json:"organizationId"`
		} `json:"organizationIdQuery"`
	}, 1)
	payload.Queries[0].OrganizationIDQuery.OrganizationID = tenantID

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, errors.New("list ZITADEL users: encode request failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/users", bytes.NewReader(body))
	if err != nil {
		return nil, 0, errors.New("list ZITADEL users: create request failed")
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, 0, fmt.Errorf("list ZITADEL users: %w", ctxErr)
		}
		return nil, 0, errors.New("list ZITADEL users: request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, 0, fmt.Errorf("list ZITADEL users: directory returned HTTP status %d", response.StatusCode)
	}

	var result struct {
		Details struct {
			TotalResult json.RawMessage `json:"totalResult"`
		} `json:"details"`
		Result []struct {
			UserID  string `json:"userId"`
			Details struct {
				ResourceOwner string `json:"resourceOwner"`
			} `json:"details"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, 0, errors.New("list ZITADEL users: decode response failed")
	}
	total, err := parseTotalResult(result.Details.TotalResult)
	if err != nil {
		return nil, 0, errors.New("list ZITADEL users: invalid response total result")
	}
	users := make([]User, 0, len(result.Result))
	for _, row := range result.Result {
		if row.Details.ResourceOwner != tenantID {
			return nil, 0, errors.New("list ZITADEL users: response resource owner does not match request tenant")
		}
		users = append(users, User{Subject: row.UserID, TenantID: row.Details.ResourceOwner})
	}
	return users, total, nil
}

func parseTotalResult(raw json.RawMessage) (uint64, error) {
	if len(raw) == 0 {
		return 0, errors.New("missing total result")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return 0, err
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	default:
		return 0, errors.New("unsupported total result type")
	}
	return strconv.ParseUint(text, 10, 64)
}
