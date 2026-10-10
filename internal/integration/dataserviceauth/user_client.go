package dataserviceauth

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/dataservice"
	"time"
)

// ActiveUserClient uses the official User v2 GetUserByID REST projection:
// https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.GetUserByID
// Only status and exact ID are consumed; no profile or service token is retained.
type ActiveUserClient struct {
	base   string
	client *http.Client
}

func NewActiveUserClient(base string, client *http.Client) (*ActiveUserClient, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, dataservice.ErrUnavailable
	}
	local := u.Hostname() == "localhost"
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		local = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, dataservice.ErrUnavailable
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	clone := *client
	clone.Timeout = 5 * time.Second
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &ActiveUserClient{strings.TrimRight(base, "/"), &clone}, nil
}
func (c *ActiveUserClient) IsUserActive(ctx context.Context, token, actor string) (bool, error) {
	if c == nil || token == "" || strings.ContainsAny(token, "\r\n") || !authidentity.IsBoundedIdentifier(actor) {
		return false, dataservice.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v2/users/"+url.PathEscape(actor), nil)
	if err != nil {
		return false, dataservice.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return false, dataservice.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, dataservice.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 || ctx.Err() != nil {
		return false, dataservice.ErrUnavailable
	}
	var result struct {
		User *struct {
			ID    string          `json:"userId"`
			State json.RawMessage `json:"state"`
		} `json:"user"`
	}
	if json.Unmarshal(raw, &result) != nil || result.User == nil || result.User.ID != actor {
		return false, dataservice.ErrUnavailable
	}
	switch string(result.User.State) {
	case `"USER_STATE_ACTIVE"`, "1":
		return true, nil
	case `"USER_STATE_INACTIVE"`, `"USER_STATE_DELETED"`, `"USER_STATE_LOCKED"`, "2", "3", "4":
		return false, nil
	default:
		return false, dataservice.ErrUnavailable
	}
}
