// Package accountaudit projects the current registry's read-only history into
// the Account transport contract. Registry remains the fact/permission owner.
package accountaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"time"

	"task-processor/internal/authidentity"
	registry "task-processor/internal/sourceaccountregistry"
)

type History interface {
	List(context.Context, registry.HistoryRequest) (registry.HistoryPage, error)
}
type Query struct{ history History }

func New(history History) (*Query, error) {
	if history == nil || reflect.ValueOf(history).Kind() == reflect.Ptr && reflect.ValueOf(history).IsNil() {
		return nil, registry.ErrUnavailable
	}
	return &Query{history: history}, nil
}

type Relation struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Version   string `json:"version"`
}
type Event struct {
	EventType       string    `json:"eventType"`
	Actor           string    `json:"actor"`
	Time            time.Time `json:"time"`
	ObjectType      string    `json:"objectType"`
	ObjectReference string    `json:"objectReference"`
	Operation       string    `json:"operation"`
	Result          string    `json:"result"`
	Relation        Relation  `json:"relation"`
}
type Page struct {
	SchemaVersion           string  `json:"schemaVersion"`
	UserID                  string  `json:"userId"`
	EffectiveOrganizationID string  `json:"effectiveOrganizationId"`
	Source                  string  `json:"source"`
	Items                   []Event `json:"items"`
	NextCursor              *string `json:"nextCursor"`
}
type positionWire struct {
	Organization string    `json:"org"`
	Time         time.Time `json:"time"`
	Account      string    `json:"account"`
	Version      string    `json:"version"`
}

func (q *Query) Read(ctx context.Context, limit int, cursor string) (Page, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return Page{}, registry.ErrAuthenticationRequired
	}
	if identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID {
		return Page{}, registry.ErrForbidden
	}
	after, err := parseCursor(cursor, identity.EffectiveOrganizationID)
	if err != nil {
		return Page{}, err
	}
	request := registry.HistoryRequest{Limit: limit, After: after}
	if request.Validate() != nil {
		return Page{}, registry.ErrInvalid
	}
	history, err := q.history.List(ctx, request)
	if err != nil {
		return Page{}, err
	}
	if ctx.Err() != nil {
		return Page{}, ctx.Err()
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return Page{}, registry.ErrAuthenticationRequired
	}
	if len(history.Items) > limit {
		return Page{}, registry.ErrUnavailable
	}
	result := Page{SchemaVersion: "account-audit-v1", UserID: identity.UserID, EffectiveOrganizationID: identity.EffectiveOrganizationID, Source: "source_account_committed_operations", Items: make([]Event, 0, len(history.Items))}
	previous := after
	for _, item := range history.Items {
		p := item.Position()
		if item.Validate() != nil || item.OrganizationID != identity.EffectiveOrganizationID || previous != nil && !p.Before(*previous) {
			return Page{}, registry.ErrUnavailable
		}
		previous = &p
		result.Items = append(result.Items, Event{EventType: "source_account.operation_committed", Actor: item.ActorSubject, Time: item.OccurredAt.UTC(), ObjectType: "source_account", ObjectReference: item.AccountID, Operation: string(item.Kind), Result: "succeeded", Relation: Relation{Type: "source_account_version", Reference: item.AccountID, Version: strconv.FormatInt(item.Version, 10)}})
	}
	if history.Next != nil {
		if len(history.Items) != limit || previous == nil || history.Next.Validate() != nil || !history.Next.Equal(*previous) {
			return Page{}, registry.ErrUnavailable
		}
		data, err := json.Marshal(positionWire{Organization: identity.EffectiveOrganizationID, Time: history.Next.OccurredAt.UTC(), Account: history.Next.AccountID, Version: strconv.FormatInt(history.Next.Version, 10)})
		if err != nil {
			return Page{}, registry.ErrUnavailable
		}
		value := base64.RawURLEncoding.EncodeToString(data)
		result.NextCursor = &value
	}
	return result, nil
}
func parseCursor(value, organization string) (*registry.HistoryPosition, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 2048 {
		return nil, registry.ErrInvalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, registry.ErrInvalid
	}
	var wire positionWire
	if json.Unmarshal(data, &wire) != nil || wire.Organization != organization {
		return nil, registry.ErrInvalid
	}
	version, err := strconv.ParseInt(wire.Version, 10, 64)
	if err != nil || strconv.FormatInt(version, 10) != wire.Version {
		return nil, registry.ErrInvalid
	}
	position := registry.HistoryPosition{OccurredAt: wire.Time, AccountID: wire.Account, Version: version}
	if position.Validate() != nil {
		return nil, registry.ErrInvalid
	}
	// A canonical re-encoding rejects unknown/duplicate fields, alternate JSON
	// spellings, trailing data and noncanonical encodings without retaining input.
	canonical, err := json.Marshal(wire)
	if err != nil || base64.RawURLEncoding.EncodeToString(canonical) != value {
		return nil, registry.ErrInvalid
	}
	return &position, nil
}
