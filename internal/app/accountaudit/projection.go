// Package accountaudit projects the current registry's read-only history into
// the Account transport contract. Registry remains the fact/permission owner.
package accountaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"time"

	allocation "task-processor/internal/accountallocation"
	"task-processor/internal/authidentity"
	registry "task-processor/internal/sourceaccountregistry"
)

type History interface {
	List(context.Context, registry.HistoryRequest) (registry.HistoryPage, error)
}
type AllocationHistory interface {
	ListRecentAudit(context.Context, string, int, string, string) ([]allocation.AuditEvent, error)
}
type Query struct {
	history    History
	allocation AllocationHistory
}
type Filter struct {
	ActorSubject      string
	Kind              registry.OperationKind
	ResourceOperation string
}

func New(history History) (*Query, error) {
	if history == nil || reflect.ValueOf(history).Kind() == reflect.Ptr && reflect.ValueOf(history).IsNil() {
		return nil, registry.ErrUnavailable
	}
	return &Query{history: history}, nil
}

func NewWithAllocation(history History, allocationHistory AllocationHistory) (*Query, error) {
	query, err := New(history)
	if err != nil {
		return nil, err
	}
	query.allocation = allocationHistory
	return query, nil
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
	Actor        string    `json:"actor,omitempty"`
	Kind         string    `json:"kind,omitempty"`
}

func (q *Query) Read(ctx context.Context, limit int, cursor string) (Page, error) {
	return q.ReadFiltered(ctx, limit, cursor, Filter{})
}

func (q *Query) ReadFiltered(ctx context.Context, limit int, cursor string, filter Filter) (Page, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return Page{}, registry.ErrAuthenticationRequired
	}
	if identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID {
		return Page{}, registry.ErrForbidden
	}
	after, err := parseCursor(cursor, identity.EffectiveOrganizationID, filter)
	if err != nil {
		return Page{}, err
	}
	allocationItems := []allocation.AuditEvent{}
	if q.allocation != nil && cursor == "" {
		var allocationErr error
		allocationItems, allocationErr = q.allocation.ListRecentAudit(ctx, identity.EffectiveOrganizationID, limit, filter.ActorSubject, filter.ResourceOperation)
		if allocationErr != nil {
			return Page{}, allocationErr
		}
	}
	sourceLimit := limit - len(allocationItems)
	if sourceLimit < 0 {
		sourceLimit = 0
	}
	if sourceLimit < 1 && len(allocationItems) == 0 {
		sourceLimit = limit
	}
	history := registry.HistoryPage{}
	if filter.ResourceOperation == "" && sourceLimit > 0 {
		request := registry.HistoryRequest{Limit: sourceLimit, After: after, ActorSubject: filter.ActorSubject, Kind: filter.Kind}
		if request.Validate() != nil {
			return Page{}, registry.ErrInvalid
		}
		var historyErr error
		history, historyErr = q.history.List(ctx, request)
		if historyErr != nil {
			return Page{}, historyErr
		}
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
	result := Page{SchemaVersion: "account-audit-v1", UserID: identity.UserID, EffectiveOrganizationID: identity.EffectiveOrganizationID, Source: "source_account_committed_operations", Items: make([]Event, 0, len(history.Items)+len(allocationItems))}
	previous := after
	for _, item := range history.Items {
		p := item.Position()
		if item.Validate() != nil || item.OrganizationID != identity.EffectiveOrganizationID || previous != nil && !p.Before(*previous) {
			return Page{}, registry.ErrUnavailable
		}
		previous = &p
		result.Items = append(result.Items, Event{EventType: "source_account.operation_committed", Actor: item.ActorSubject, Time: item.OccurredAt.UTC(), ObjectType: "source_account", ObjectReference: item.AccountID, Operation: string(item.Kind), Result: "succeeded", Relation: Relation{Type: "source_account_version", Reference: item.AccountID, Version: strconv.FormatInt(item.Version, 10)}})
	}
	for _, item := range allocationItems {
		if item.OrganizationID != identity.EffectiveOrganizationID || item.ActorID == "" || item.MemberID == "" || item.Version < 1 || item.CreatedAt.IsZero() {
			return Page{}, registry.ErrUnavailable
		}
		result.Items = append(result.Items, Event{EventType: "account_member_token_allocation.changed", Actor: item.ActorID, Time: item.CreatedAt.UTC(), ObjectType: "member_token_allocation", ObjectReference: item.MemberID, Operation: item.Operation, Result: "succeeded", Relation: Relation{Type: "member_token_allocation_version", Reference: item.MemberID, Version: strconv.FormatInt(item.Version, 10)}})
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].Time.After(result.Items[j].Time) })
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	if len(allocationItems) > 0 {
		result.Source = "source_account_committed_operations+account_member_token_audit"
	}
	if history.Next != nil {
		if len(history.Items) != sourceLimit || previous == nil || history.Next.Validate() != nil || !history.Next.Equal(*previous) {
			return Page{}, registry.ErrUnavailable
		}
		data, err := json.Marshal(positionWire{Organization: identity.EffectiveOrganizationID, Time: history.Next.OccurredAt.UTC(), Account: history.Next.AccountID, Version: strconv.FormatInt(history.Next.Version, 10), Actor: filter.ActorSubject, Kind: string(filter.Kind)})
		if err != nil {
			return Page{}, registry.ErrUnavailable
		}
		value := base64.RawURLEncoding.EncodeToString(data)
		result.NextCursor = &value
	}
	return result, nil
}
func parseCursor(value, organization string, filter Filter) (*registry.HistoryPosition, error) {
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
	if json.Unmarshal(data, &wire) != nil || wire.Organization != organization || wire.Actor != filter.ActorSubject || wire.Kind != string(filter.Kind) {
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
