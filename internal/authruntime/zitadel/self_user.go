package zitadel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/authidentity"
	"time"
)

type SelfUserFacts struct {
	RegisteredAt      time.Time
	LastLogin         *time.Time
	PasswordChangedAt *time.Time
	Profile           authidentity.SelfProfile
}

func (c *SelfServiceClient) ReadUserFacts(ctx context.Context, token, subject string) (SelfUserFacts, error) {
	var empty SelfUserFacts
	if c == nil || c.baseURL == "" || c.client == nil || strings.TrimSpace(token) == "" || !authidentity.IsBoundedIdentifier(subject) {
		return empty, errors.New("self user facts unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return empty, errors.New("self user facts unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return empty, errors.New("self user facts unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return empty, &SelfServiceError{StatusCode: response.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16*1024+1))
	if err != nil || len(raw) > 16*1024 || ctx.Err() != nil {
		return empty, errors.New("self user facts invalid")
	}
	var payload struct {
		User *struct {
			ID      string `json:"id"`
			Details struct {
				CreationDate string `json:"creationDate"`
			} `json:"details"`
			Human *struct {
				Profile struct {
					DisplayName string `json:"displayName"`
				} `json:"profile"`
				Email *struct {
					Email    string `json:"email"`
					Verified bool   `json:"isEmailVerified"`
				} `json:"email"`
				Phone *struct {
					Phone    string `json:"phone"`
					Verified bool   `json:"isPhoneVerified"`
				} `json:"phone"`
				PasswordChanged string `json:"passwordChanged"`
			} `json:"human"`
		} `json:"user"`
		LastLogin string `json:"lastLogin"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.User == nil || payload.User.ID != subject || payload.User.Human == nil {
		return empty, errors.New("self user facts invalid")
	}
	registered, err := time.Parse(time.RFC3339Nano, payload.User.Details.CreationDate)
	if err != nil || registered.IsZero() {
		return empty, errors.New("self user facts invalid")
	}
	result := SelfUserFacts{RegisteredAt: registered.UTC(), Profile: authidentity.SelfProfile{UserID: subject}}
	parseOptional := func(value string) (*time.Time, error) {
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || parsed.IsZero() {
			return nil, errors.New("self user date invalid")
		}
		utc := parsed.UTC()
		return &utc, nil
	}
	result.LastLogin, err = parseOptional(payload.LastLogin)
	if err != nil {
		return empty, err
	}
	result.PasswordChangedAt, err = parseOptional(payload.User.Human.PasswordChanged)
	if err != nil {
		return empty, err
	}
	human := payload.User.Human
	if human.Profile.DisplayName != "" {
		result.Profile.DisplayName = &human.Profile.DisplayName
	}
	if human.Email != nil && human.Email.Email != "" {
		result.Profile.Email = &human.Email.Email
		result.Profile.EmailVerified = &human.Email.Verified
	}
	if human.Phone != nil && human.Phone.Phone != "" {
		result.Profile.PhoneNumber = &human.Phone.Phone
		result.Profile.PhoneNumberVerified = &human.Phone.Verified
	}
	return result, nil
}
