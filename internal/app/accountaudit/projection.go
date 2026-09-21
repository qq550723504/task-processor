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
	ListRecentAudit(context.Context, string, int, string, string, *allocation.AuditPosition) (allocation.AuditPage, error)
}
type AuditPosition struct {
	CreatedAt time.Time
	Key       string
}
type AdditionalAuditEvent struct {
	EventType         string
	Actor             string
	Time              time.Time
	ObjectType        string
	ObjectReference   string
	Operation         string
	Version           int64
	Key               string
	RelationReference string
}
type AdditionalAuditPage struct {
	Items []AdditionalAuditEvent
	Next  *AuditPosition
}
type AdditionalHistory interface {
	ListRecentAudit(context.Context, string, int, string, string, *AuditPosition) (AdditionalAuditPage, error)
}
type Query struct {
	history    History
	allocation AllocationHistory
	profile    AdditionalHistory
	membership AdditionalHistory
}
type Filter struct {
	ActorSubject        string
	Kind                registry.OperationKind
	ResourceOperation   string
	ProfileOperation    string
	MembershipOperation string
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

func NewWithAuditSources(history History, allocationHistory AllocationHistory, profile AdditionalHistory, membership AdditionalHistory) (*Query, error) {
	query, err := NewWithAllocation(history, allocationHistory)
	if err != nil {
		return nil, err
	}
	query.profile = profile
	query.membership = membership
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
	Organization        string            `json:"org"`
	Source              *sourcePosition   `json:"source,omitempty"`
	Allocation          *allocationCursor `json:"allocation,omitempty"`
	Actor               string            `json:"actor,omitempty"`
	Kind                string            `json:"kind,omitempty"`
	ResourceOperation   string            `json:"resourceOperation,omitempty"`
	ProfileOperation    string            `json:"profileOperation,omitempty"`
	MembershipOperation string            `json:"membershipOperation,omitempty"`
	Profile             *auditCursor      `json:"profile,omitempty"`
	Membership          *auditCursor      `json:"membership,omitempty"`
}

type sourcePosition struct {
	Time    time.Time `json:"time"`
	Account string    `json:"account"`
	Version string    `json:"version"`
}

type allocationCursor struct {
	Time string `json:"time"`
	Key  string `json:"key"`
}
type auditCursor struct {
	Time string `json:"time"`
	Key  string `json:"key"`
}

type cursorState struct {
	source     *registry.HistoryPosition
	allocation *allocation.AuditPosition
	profile    *AuditPosition
	membership *AuditPosition
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
	state, err := parseCursor(cursor, identity.EffectiveOrganizationID, filter)
	if err != nil {
		return Page{}, err
	}
	allocationPage := allocation.AuditPage{}
	if q.allocation != nil && filter.Kind == "" && filter.ProfileOperation == "" && filter.MembershipOperation == "" {
		var allocationErr error
		allocationPage, allocationErr = q.allocation.ListRecentAudit(ctx, identity.EffectiveOrganizationID, limit, filter.ActorSubject, filter.ResourceOperation, state.allocation)
		if allocationErr != nil {
			return Page{}, allocationErr
		}
	}
	profilePage := AdditionalAuditPage{}
	if q.profile != nil && filter.Kind == "" && filter.ResourceOperation == "" && filter.MembershipOperation == "" {
		var profileErr error
		profilePage, profileErr = q.profile.ListRecentAudit(ctx, identity.EffectiveOrganizationID, limit, filter.ActorSubject, filter.ProfileOperation, state.profile)
		if profileErr != nil {
			return Page{}, profileErr
		}
	}
	membershipPage := AdditionalAuditPage{}
	if q.membership != nil && filter.Kind == "" && filter.ResourceOperation == "" && filter.ProfileOperation == "" {
		var membershipErr error
		membershipPage, membershipErr = q.membership.ListRecentAudit(ctx, identity.EffectiveOrganizationID, limit, filter.ActorSubject, filter.MembershipOperation, state.membership)
		if membershipErr != nil {
			return Page{}, membershipErr
		}
	}
	history := registry.HistoryPage{}
	if filter.ResourceOperation == "" && filter.ProfileOperation == "" && filter.MembershipOperation == "" {
		request := registry.HistoryRequest{Limit: limit, After: state.source, ActorSubject: filter.ActorSubject, Kind: filter.Kind}
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
	if len(history.Items) > limit || len(allocationPage.Items) > limit || len(profilePage.Items) > limit || len(membershipPage.Items) > limit {
		return Page{}, registry.ErrUnavailable
	}
	type mergedEvent struct {
		event      Event
		source     *registry.HistoryPosition
		allocation *allocation.AuditPosition
		profile    *AuditPosition
		membership *AuditPosition
		kind       string
		key        string
	}
	merged := make([]mergedEvent, 0, len(history.Items)+len(allocationPage.Items)+len(profilePage.Items)+len(membershipPage.Items))
	for _, item := range history.Items {
		p := item.Position()
		if item.Validate() != nil || item.OrganizationID != identity.EffectiveOrganizationID {
			return Page{}, registry.ErrUnavailable
		}
		merged = append(merged, mergedEvent{event: Event{EventType: "source_account.operation_committed", Actor: item.ActorSubject, Time: item.OccurredAt.UTC(), ObjectType: "source_account", ObjectReference: item.AccountID, Operation: string(item.Kind), Result: "succeeded", Relation: Relation{Type: "source_account_version", Reference: item.AccountID, Version: strconv.FormatInt(item.Version, 10)}}, source: &p, kind: "source", key: item.AccountID + ":" + strconv.FormatInt(item.Version, 10)})
	}
	for _, item := range allocationPage.Items {
		if item.OrganizationID != identity.EffectiveOrganizationID || item.ActorID == "" || item.MemberID == "" || item.Version < 1 || item.CreatedAt.IsZero() {
			return Page{}, registry.ErrUnavailable
		}
		p := item.Position()
		if !p.Valid() {
			return Page{}, registry.ErrUnavailable
		}
		merged = append(merged, mergedEvent{event: Event{EventType: "account_member_token_allocation.changed", Actor: item.ActorID, Time: item.CreatedAt.UTC(), ObjectType: "member_token_allocation", ObjectReference: item.MemberID, Operation: item.Operation, Result: "succeeded", Relation: Relation{Type: "member_token_allocation_version", Reference: item.MemberID, Version: strconv.FormatInt(item.Version, 10)}}, allocation: &p, kind: "allocation", key: item.IdempotencyKey})
	}
	for _, item := range profilePage.Items {
		if item.Actor == "" || item.ObjectReference == "" || item.Time.IsZero() || item.Version < 1 || item.EventType == "" {
			return Page{}, registry.ErrUnavailable
		}
		p := AuditPosition{CreatedAt: item.Time.UTC().Truncate(time.Microsecond), Key: item.Key}
		if p.Key == "" {
			return Page{}, registry.ErrUnavailable
		}
		merged = append(merged, mergedEvent{event: Event{EventType: item.EventType, Actor: item.Actor, Time: item.Time.UTC(), ObjectType: item.ObjectType, ObjectReference: item.ObjectReference, Operation: item.Operation, Result: "succeeded", Relation: Relation{Type: item.ObjectType + "_version", Reference: item.ObjectReference, Version: strconv.FormatInt(item.Version, 10)}}, profile: &p, kind: "profile", key: item.Key})
	}
	for _, item := range membershipPage.Items {
		if item.Actor == "" || item.ObjectReference == "" || item.Time.IsZero() || item.Version < 1 || item.EventType == "" {
			return Page{}, registry.ErrUnavailable
		}
		p := AuditPosition{CreatedAt: item.Time.UTC().Truncate(time.Microsecond), Key: item.Key}
		if p.Key == "" {
			return Page{}, registry.ErrUnavailable
		}
		relationReference := item.RelationReference
		if relationReference == "" {
			return Page{}, registry.ErrUnavailable
		}
		merged = append(merged, mergedEvent{event: Event{EventType: item.EventType, Actor: item.Actor, Time: item.Time.UTC(), ObjectType: item.ObjectType, ObjectReference: item.ObjectReference, Operation: item.Operation, Result: "succeeded", Relation: Relation{Type: "organization_membership_operation", Reference: relationReference, Version: strconv.FormatInt(item.Version, 10)}}, membership: &p, kind: "membership", key: item.Key})
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if !merged[i].event.Time.Equal(merged[j].event.Time) {
			return merged[i].event.Time.After(merged[j].event.Time)
		}
		if merged[i].kind != merged[j].kind {
			return merged[i].kind < merged[j].kind
		}
		return merged[i].key > merged[j].key
	})
	mergedTruncated := len(merged) > limit
	if mergedTruncated {
		merged = merged[:limit]
	}
	result := Page{SchemaVersion: "account-audit-v1", UserID: identity.UserID, EffectiveOrganizationID: identity.EffectiveOrganizationID, Source: "source_account_committed_operations", Items: make([]Event, 0, len(merged))}
	var nextState cursorState = state
	for _, item := range merged {
		result.Items = append(result.Items, item.event)
		if item.source != nil {
			nextState.source = item.source
		}
		if item.allocation != nil {
			nextState.allocation = item.allocation
		}
		if item.profile != nil {
			nextState.profile = item.profile
		}
		if item.membership != nil {
			nextState.membership = item.membership
		}
	}
	if len(allocationPage.Items) > 0 {
		result.Source = "source_account_committed_operations+account_member_token_audit"
	}
	if len(profilePage.Items) > 0 {
		result.Source += "+account_business_profile_audit"
	}
	if len(membershipPage.Items) > 0 {
		result.Source += "+organization_member_audit"
	}
	// Each source is fetched independently. The merged page can therefore be
	// truncated even when neither source returned its own page cursor; the
	// cursor still needs to carry the last emitted position from both streams
	// so the events beyond the merge boundary remain reachable.
	if mergedTruncated || history.Next != nil || allocationPage.Next != nil || profilePage.Next != nil || membershipPage.Next != nil {
		wire := positionWire{Organization: identity.EffectiveOrganizationID, Actor: filter.ActorSubject, Kind: string(filter.Kind), ResourceOperation: filter.ResourceOperation, ProfileOperation: filter.ProfileOperation, MembershipOperation: filter.MembershipOperation}
		if nextState.source != nil {
			wire.Source = &sourcePosition{Time: nextState.source.OccurredAt.UTC(), Account: nextState.source.AccountID, Version: strconv.FormatInt(nextState.source.Version, 10)}
		}
		if nextState.allocation != nil {
			wire.Allocation = &allocationCursor{Time: nextState.allocation.CreatedAt.UTC().Format(time.RFC3339Nano), Key: nextState.allocation.IdempotencyKey}
		}
		if nextState.profile != nil {
			wire.Profile = &auditCursor{Time: nextState.profile.CreatedAt.UTC().Format(time.RFC3339Nano), Key: nextState.profile.Key}
		}
		if nextState.membership != nil {
			wire.Membership = &auditCursor{Time: nextState.membership.CreatedAt.UTC().Format(time.RFC3339Nano), Key: nextState.membership.Key}
		}
		data, err := json.Marshal(wire)
		if err != nil {
			return Page{}, registry.ErrUnavailable
		}
		value := base64.RawURLEncoding.EncodeToString(data)
		result.NextCursor = &value
	}
	return result, nil
}
func parseCursor(value, organization string, filter Filter) (cursorState, error) {
	if value == "" {
		return cursorState{}, nil
	}
	if len(value) > 2048 {
		return cursorState{}, registry.ErrInvalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return cursorState{}, registry.ErrInvalid
	}
	var wire positionWire
	if json.Unmarshal(data, &wire) != nil || wire.Organization != organization || wire.Actor != filter.ActorSubject || wire.Kind != string(filter.Kind) || wire.ResourceOperation != filter.ResourceOperation || wire.ProfileOperation != filter.ProfileOperation || wire.MembershipOperation != filter.MembershipOperation || wire.Source == nil && wire.Allocation == nil && wire.Profile == nil && wire.Membership == nil {
		return cursorState{}, registry.ErrInvalid
	}
	state := cursorState{}
	if wire.Source != nil {
		version, err := strconv.ParseInt(wire.Source.Version, 10, 64)
		if err != nil || strconv.FormatInt(version, 10) != wire.Source.Version {
			return cursorState{}, registry.ErrInvalid
		}
		position := registry.HistoryPosition{OccurredAt: wire.Source.Time, AccountID: wire.Source.Account, Version: version}
		if position.Validate() != nil {
			return cursorState{}, registry.ErrInvalid
		}
		state.source = &position
	}
	if wire.Allocation != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Allocation.Time)
		if err != nil {
			return cursorState{}, registry.ErrInvalid
		}
		position := allocation.AuditPosition{CreatedAt: createdAt, IdempotencyKey: wire.Allocation.Key}
		if !position.Valid() {
			return cursorState{}, registry.ErrInvalid
		}
		state.allocation = &position
	}
	if wire.Profile != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Profile.Time)
		if err != nil || wire.Profile.Key == "" {
			return cursorState{}, registry.ErrInvalid
		}
		state.profile = &AuditPosition{CreatedAt: createdAt, Key: wire.Profile.Key}
	}
	if wire.Membership != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Membership.Time)
		if err != nil || wire.Membership.Key == "" {
			return cursorState{}, registry.ErrInvalid
		}
		state.membership = &AuditPosition{CreatedAt: createdAt, Key: wire.Membership.Key}
	}
	// A canonical re-encoding rejects unknown/duplicate fields, alternate JSON
	// spellings, trailing data and noncanonical encodings without retaining input.
	canonical, err := json.Marshal(wire)
	if err != nil || base64.RawURLEncoding.EncodeToString(canonical) != value {
		return cursorState{}, registry.ErrInvalid
	}
	return state, nil
}
