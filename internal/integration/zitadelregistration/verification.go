package zitadelregistration

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/referral"
)

func (c *Client) Read(ctx context.Context, subject string) (app.User, error) {
	if subject == "" || len(subject) > 200 {
		return app.User{}, referral.ErrInvalid
	}
	path := "/v2/users/" + url.PathEscape(subject)
	var response struct {
		User struct {
			ID      string `json:"userId"`
			Details struct {
				Organization string `json:"resourceOwner"`
			}
			Human *struct {
				Email struct {
					Email    string
					Verified bool `json:"isVerified"`
				}
			}
		}
	}
	if err := c.request(ctx, http.MethodGet, path, nil, &response); err != nil {
		return app.User{}, err
	}
	u := response.User
	if u.ID != subject || u.Details.Organization != c.config.Organization || u.Human == nil || u.Human.Email.Email == "" {
		return app.User{}, referral.ErrUnknown
	}
	var metadata struct{ Metadata []struct{ Key, Value string } }
	if err := c.request(ctx, http.MethodPost, path+"/metadata/search", map[string]any{"pagination": map[string]int{"limit": 100}}, &metadata); err != nil {
		return app.User{}, referral.ErrUnknown
	}
	proof := ""
	for _, entry := range metadata.Metadata {
		if entry.Key != proofKey {
			continue
		}
		value, err := base64.StdEncoding.DecodeString(entry.Value)
		if err != nil || len(value) == 0 || len(value) > 256 || proof != "" {
			return app.User{}, referral.ErrUnknown
		}
		proof = string(value)
	}
	if proof == "" {
		return app.User{}, referral.ErrUnknown
	}
	return app.User{Subject: u.ID, Organization: u.Details.Organization, Email: u.Human.Email.Email, Verified: u.Human.Email.Verified, Proof: proof}, nil
}
