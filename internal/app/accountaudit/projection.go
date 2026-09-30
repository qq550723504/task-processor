// Package accountaudit projects read-only history from current domain owners
// into the Account transport contract. Each source remains its fact owner.
package accountaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"

	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
)

var auditSearchFold = cases.Fold()

type History interface {
	List(context.Context, registry.HistoryRequest) (registry.HistoryPage, error)
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
	Resource          *ResourceDetail
}
type AdditionalAuditPage struct {
	Items []AdditionalAuditEvent
	Next  *AuditPosition
}
type AdditionalHistory interface {
	ListRecentAudit(context.Context, string, int, string, string, *AuditPosition) (AdditionalAuditPage, error)
}
type UsageAuditEvent struct {
	OrganizationID, EventID, MemberID, InvocationID string
	Quantity                                        int64
	Time                                            time.Time
}
type UsageAuditPage struct {
	Items []UsageAuditEvent
	Next  *AuditPosition
}
type UsageHistory interface {
	ListObservedAIUsageAudit(context.Context, string, int, *AuditPosition) (UsageAuditPage, error)
	// Complete reports whether all current AI usage namespaces can be read.
	Complete() bool
}
type Query struct {
	history    History
	profile    AdditionalHistory
	membership AdditionalHistory
	usage      UsageHistory
	points     ImagePointHistory
	resources  AdditionalHistory
}
type ImagePointHistory interface {
	ListImagePointDebits(context.Context, string, string, int, *orgresource.ImagePointAuditPosition) (orgresource.ImagePointAuditPage, error)
}
type Filter struct {
	ActorSubject        string
	Kind                registry.OperationKind
	ResourceOperation   string
	ProfileOperation    string
	MembershipOperation string
	Content             string
	MemberID            string
	Period              string
	AsOf                string
}

func New(history History) (*Query, error) {
	if history == nil || reflect.ValueOf(history).Kind() == reflect.Ptr && reflect.ValueOf(history).IsNil() {
		return nil, registry.ErrUnavailable
	}
	return &Query{history: history}, nil
}

// NewCurrentAuditSources consumes only current native and resource owners.
func NewCurrentAuditSources(history History, profile, membership AdditionalHistory, usage UsageHistory, points ImagePointHistory, resources AdditionalHistory) (*Query, error) {
	query, err := New(history)
	if err != nil {
		return nil, err
	}
	query.profile, query.membership, query.usage, query.points = profile, membership, usage, points
	query.resources = resources
	return query, nil
}

type Relation struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Version   string `json:"version"`
}
type Event struct {
	EventType       string          `json:"eventType"`
	Actor           string          `json:"actor"`
	Time            time.Time       `json:"time"`
	ObjectType      string          `json:"objectType"`
	ObjectReference string          `json:"objectReference"`
	Operation       string          `json:"operation"`
	Result          string          `json:"result"`
	Relation        Relation        `json:"relation"`
	Usage           *UsageDetail    `json:"usage,omitempty"`
	Points          *PointDetail    `json:"points,omitempty"`
	Resource        *ResourceDetail `json:"resource,omitempty"`
}
type ResourceDetail struct {
	Type     orgresource.ResourceType `json:"type"`
	Quantity string                   `json:"quantity"`
}
type PointDetail struct {
	MemberID     string `json:"memberId"`
	Quantity     string `json:"quantity"`
	PriceVersion string `json:"priceVersion"`
	IntentID     string `json:"intentId"`
}
type UsageDetail struct {
	MemberID string `json:"memberId"`
	Quantity int64  `json:"quantity"`
	Metric   string `json:"metric"`
}
type Page struct {
	SchemaVersion           string  `json:"schemaVersion"`
	UserID                  string  `json:"userId"`
	EffectiveOrganizationID string  `json:"effectiveOrganizationId"`
	Source                  string  `json:"source"`
	Items                   []Event `json:"items"`
	NextCursor              *string `json:"nextCursor"`
	positions               []string
}
type positionWire struct {
	Organization        string          `json:"org"`
	Source              *sourcePosition `json:"source,omitempty"`
	Actor               string          `json:"actor,omitempty"`
	Kind                string          `json:"kind,omitempty"`
	ResourceOperation   string          `json:"resourceOperation,omitempty"`
	ProfileOperation    string          `json:"profileOperation,omitempty"`
	MembershipOperation string          `json:"membershipOperation,omitempty"`
	Content             string          `json:"query,omitempty"`
	MemberID            string          `json:"member,omitempty"`
	Period              string          `json:"period,omitempty"`
	AsOf                string          `json:"asOf,omitempty"`
	Profile             *auditCursor    `json:"profile,omitempty"`
	Membership          *auditCursor    `json:"membership,omitempty"`
	Usage               *auditCursor    `json:"usage,omitempty"`
	Points              *auditCursor    `json:"points,omitempty"`
	Resources           *auditCursor    `json:"resources,omitempty"`
}

type sourcePosition struct {
	Time    time.Time `json:"time"`
	Account string    `json:"account"`
	Version string    `json:"version"`
}

type auditCursor struct {
	Time string `json:"time"`
	Key  string `json:"key"`
}

type cursorState struct {
	source     *registry.HistoryPosition
	profile    *AuditPosition
	membership *AuditPosition
	usage      *AuditPosition
	points     *orgresource.ImagePointAuditPosition
	resources  *AuditPosition
}

func (q *Query) Read(ctx context.Context, limit int, cursor string) (Page, error) {
	return q.ReadFiltered(ctx, limit, cursor, Filter{})
}

func (q *Query) ReadFiltered(ctx context.Context, limit int, cursor string, filter Filter) (Page, error) {
	if limit < 1 || limit > registry.MaxPageLimit {
		return Page{}, registry.ErrInvalid
	}
	filter.Content = strings.TrimSpace(filter.Content)
	if len(filter.Content) > 80 || !utf8.ValidString(filter.Content) || filter.MemberID != "" && !authidentity.IsBoundedIdentifier(filter.MemberID) {
		return Page{}, registry.ErrInvalid
	}
	for _, ch := range filter.Content {
		if unicode.IsControl(ch) {
			return Page{}, registry.ErrInvalid
		}
	}
	switch filter.Period {
	case "", "7d", "30d", "all":
	default:
		return Page{}, registry.ErrInvalid
	}
	active := filter.Content != "" || filter.MemberID != "" || filter.Period != ""
	if !active {
		return q.readBatch(ctx, limit, cursor, filter)
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return Page{}, registry.ErrAuthenticationRequired
	}
	if identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID {
		return Page{}, registry.ErrForbidden
	}
	if q == nil || summaryMissing(q.history) || summaryMissing(q.profile) || summaryMissing(q.membership) || summaryMissing(q.usage) || !q.usage.Complete() || summaryMissing(q.points) || summaryMissing(q.resources) {
		return Page{}, registry.ErrUnavailable
	}
	if cursor == "" {
		filter.AsOf = time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	} else {
		wire, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
		filter.AsOf = wire.AsOf
	}
	asOf, err := time.Parse(time.RFC3339Nano, filter.AsOf)
	if err != nil || asOf.IsZero() {
		return Page{}, registry.ErrInvalid
	}
	var result Page
	var from time.Time
	if filter.Period == "7d" {
		from = asOf.Add(-7 * 24 * time.Hour)
	}
	if filter.Period == "30d" {
		from = asOf.Add(-30 * 24 * time.Hour)
	}
	var lastMatchPosition string
	position := cursor
	for {
		batch, err := q.readBatch(ctx, registry.MaxPageLimit, position, filter)
		if err != nil {
			return Page{}, err
		}
		if result.SchemaVersion == "" {
			result = Page{SchemaVersion: batch.SchemaVersion, UserID: batch.UserID, EffectiveOrganizationID: batch.EffectiveOrganizationID, Source: batch.Source, Items: []Event{}}
		}
		for index, event := range batch.Items {
			if !from.IsZero() && event.Time.Before(from) {
				return result, nil
			}
			if !matchesAuditEvent(event, filter, asOf) {
				continue
			}
			if len(result.Items) == limit {
				result.NextCursor = &lastMatchPosition
				return result, nil
			}
			result.Items = append(result.Items, event)
			lastMatchPosition = batch.positions[index]
		}
		if batch.NextCursor == nil {
			return result, nil
		}
		if len(batch.Items) == 0 || *batch.NextCursor == position {
			return Page{}, registry.ErrUnavailable
		}
		position = *batch.NextCursor
	}
}

func matchesAuditEvent(event Event, filter Filter, asOf time.Time) bool {
	if !event.Time.Before(asOf) {
		return false
	}
	if filter.Period == "7d" && event.Time.Before(asOf.Add(-7*24*time.Hour)) || filter.Period == "30d" && event.Time.Before(asOf.Add(-30*24*time.Hour)) {
		return false
	}
	if filter.MemberID != "" {
		member := ""
		switch event.EventType {
		case "account_business_profile.updated", "organization_membership.changed", "account_member_resource.changed", "account_member_ai_point_limit.changed":
			member = event.ObjectReference
		case "ai_invocation.usage_observed":
			if event.Usage != nil {
				member = event.Usage.MemberID
			}
		case "account_ai_points.committed":
			if event.Points != nil {
				member = event.Points.MemberID
			}
		}
		if member != filter.MemberID {
			return false
		}
	}
	if filter.Content == "" {
		return true
	}
	fields := []string{event.ObjectType, event.ObjectReference, event.Operation, event.Relation.Reference}
	switch event.EventType {
	case "source_account.operation_committed":
		fields = append(fields, "源账号", "源账号 "+event.ObjectReference, map[string]string{"register": "登记源账号", "enable": "启用源账号", "disable": "停用源账号"}[event.Operation])
	case "account_business_profile.updated":
		fields = append(fields, "账户资料", "账户资料 "+event.ObjectReference, "更新账户资料")
	case "organization_membership.changed":
		fields = append(fields, "成员与权限", "成员 "+event.ObjectReference, map[string]string{"invite": "邀请成员", "role": "更新成员角色", "remove": "移除成员"}[event.Operation])
	case "account_member_resource.changed", "account_member_ai_point_limit.changed":
		fields = append(fields, "资源与额度", "成员 "+event.ObjectReference, event.Resource.Quantity, string(event.Resource.Type))
		if event.EventType == "account_member_ai_point_limit.changed" {
			fields = append(fields, "设置成员 AI 月度上限", "设置成员 AI 月度上限："+event.Resource.Quantity+" 点/月")
		} else {
			verb := map[string]string{"allocate_member_resource": "分配", "reclaim_member_resource": "回收"}[event.Operation]
			if event.Resource.Type == orgresource.ResourceStoreRenewalPeriod {
				fields = append(fields, "续费期数", verb+"续费期数："+event.Resource.Quantity+" 期")
			} else {
				fields = append(fields, "数据额度", verb+"数据额度："+event.Resource.Quantity+" 条")
			}
		}
	case "ai_invocation.usage_observed":
		fields = append(fields, "资源与额度", "模型实际用量", "模型实际用量："+strconv.FormatInt(event.Usage.Quantity, 10)+" Token", "成员 "+event.Usage.MemberID)
	case "account_ai_points.committed":
		fields = append(fields, "资源与额度", "图片 AI 点数已扣", "图片 AI 点数已扣："+event.Points.Quantity, "成员 "+event.Points.MemberID)
	}
	needle := auditSearchFold.String(filter.Content)
	for _, field := range fields {
		if strings.Contains(auditSearchFold.String(field), needle) {
			return true
		}
	}
	return false
}

func (q *Query) readBatch(ctx context.Context, limit int, cursor string, filter Filter) (Page, error) {
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
	resourcePage := AdditionalAuditPage{}
	if q.resources != nil && filter.Kind == "" && filter.ProfileOperation == "" && filter.MembershipOperation == "" {
		resourcePage, err = q.resources.ListRecentAudit(ctx, identity.EffectiveOrganizationID, limit, filter.ActorSubject, filter.ResourceOperation, state.resources)
		if err != nil {
			return Page{}, err
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
	usagePage := UsageAuditPage{}
	if q.usage != nil && filter.ActorSubject == "" && filter.Kind == "" && filter.ResourceOperation == "" && filter.ProfileOperation == "" && filter.MembershipOperation == "" {
		usagePage, err = q.usage.ListObservedAIUsageAudit(ctx, identity.EffectiveOrganizationID, limit, state.usage)
		if err != nil {
			return Page{}, err
		}
	}
	pointPage := orgresource.ImagePointAuditPage{}
	if q.points != nil && filter.Kind == "" && filter.ResourceOperation == "" && filter.ProfileOperation == "" && filter.MembershipOperation == "" {
		pointPage, err = q.points.ListImagePointDebits(ctx, identity.EffectiveOrganizationID, filter.ActorSubject, limit, state.points)
		if err != nil {
			return Page{}, err
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
	if len(history.Items) > limit || len(resourcePage.Items) > limit || len(profilePage.Items) > limit || len(membershipPage.Items) > limit || len(usagePage.Items) > limit || len(pointPage.Items) > limit {
		return Page{}, registry.ErrUnavailable
	}
	type mergedEvent struct {
		event      Event
		source     *registry.HistoryPosition
		profile    *AuditPosition
		membership *AuditPosition
		usage      *AuditPosition
		points     *orgresource.ImagePointAuditPosition
		resources  *AuditPosition
		kind       string
		key        string
	}
	merged := make([]mergedEvent, 0, len(history.Items)+len(resourcePage.Items)+len(profilePage.Items)+len(membershipPage.Items)+len(usagePage.Items)+len(pointPage.Items))
	for _, item := range history.Items {
		p := item.Position()
		if item.Validate() != nil || item.OrganizationID != identity.EffectiveOrganizationID {
			return Page{}, registry.ErrUnavailable
		}
		merged = append(merged, mergedEvent{event: Event{EventType: "source_account.operation_committed", Actor: item.ActorSubject, Time: item.OccurredAt.UTC(), ObjectType: "source_account", ObjectReference: item.AccountID, Operation: string(item.Kind), Result: "succeeded", Relation: Relation{Type: "source_account_version", Reference: item.AccountID, Version: strconv.FormatInt(item.Version, 10)}}, source: &p, kind: "source", key: item.AccountID + ":" + strconv.FormatInt(item.Version, 10)})
	}
	for _, item := range resourcePage.Items {
		if item.Actor == "" || item.ObjectReference == "" || item.Time.IsZero() || item.Version < 1 || item.Key == "" || item.RelationReference == "" || !validResourceEvent(item) || filter.ActorSubject != "" && item.Actor != filter.ActorSubject || filter.ResourceOperation != "" && item.Operation != filter.ResourceOperation {
			return Page{}, registry.ErrUnavailable
		}
		p := AuditPosition{CreatedAt: item.Time.UTC().Truncate(time.Microsecond), Key: item.Key}
		merged = append(merged, mergedEvent{event: Event{EventType: item.EventType, Actor: item.Actor, Time: item.Time.UTC(), ObjectType: item.ObjectType, ObjectReference: item.ObjectReference, Operation: item.Operation, Result: "succeeded", Relation: Relation{Type: "organization_resource_operation", Reference: item.RelationReference, Version: strconv.FormatInt(item.Version, 10)}, Resource: item.Resource}, resources: &p, kind: "resources", key: item.Key})
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
	for _, item := range usagePage.Items {
		if item.OrganizationID != identity.EffectiveOrganizationID || item.EventID == "" || item.MemberID == "" || item.InvocationID == "" || item.Quantity <= 0 || item.Time.IsZero() {
			return Page{}, registry.ErrUnavailable
		}
		p := AuditPosition{CreatedAt: item.Time.UTC().Truncate(time.Microsecond), Key: item.EventID}
		merged = append(merged, mergedEvent{event: Event{EventType: "ai_invocation.usage_observed", Time: item.Time.UTC(), ObjectType: "ai_invocation", ObjectReference: item.InvocationID, Operation: "observe", Result: "observed", Relation: Relation{Type: "ai_invocation", Reference: item.EventID}, Usage: &UsageDetail{MemberID: item.MemberID, Quantity: item.Quantity, Metric: "model_tokens"}}, usage: &p, kind: "usage", key: item.EventID})
	}
	for _, item := range pointPage.Items {
		if item.OrganizationID != identity.EffectiveOrganizationID || item.EventID == "" || item.ActorID == "" || filter.ActorSubject != "" && item.ActorID != filter.ActorSubject || item.MemberID == "" || item.RunID == "" || item.IntentID == "" || item.PriceVersion == "" || item.Points <= 0 || item.CreatedAt.IsZero() {
			return Page{}, registry.ErrUnavailable
		}
		p := orgresource.ImagePointAuditPosition{CreatedAt: item.CreatedAt.UTC().Truncate(time.Microsecond), EventID: item.EventID}
		merged = append(merged, mergedEvent{event: Event{EventType: "account_ai_points.committed", Actor: item.ActorID, Time: item.CreatedAt.UTC(), ObjectType: "image_generation", ObjectReference: item.RunID, Operation: "consume", Result: "succeeded", Relation: Relation{Type: "organization_resource_event", Reference: item.EventID}, Points: &PointDetail{MemberID: item.MemberID, Quantity: strconv.FormatInt(item.Points, 10), PriceVersion: item.PriceVersion, IntentID: item.IntentID}}, points: &p, kind: "points", key: item.EventID})
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if !merged[i].event.Time.Equal(merged[j].event.Time) {
			return merged[i].event.Time.After(merged[j].event.Time)
		}
		if merged[i].kind != merged[j].kind {
			return merged[i].kind < merged[j].kind
		}
		if merged[i].source != nil && merged[j].source != nil {
			return merged[j].source.Before(*merged[i].source)
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
		if item.resources != nil {
			nextState.resources = item.resources
		}
		if item.profile != nil {
			nextState.profile = item.profile
		}
		if item.membership != nil {
			nextState.membership = item.membership
		}
		if item.usage != nil {
			nextState.usage = item.usage
		}
		if item.points != nil {
			nextState.points = item.points
		}
		position, positionErr := encodeCursor(nextState, identity.EffectiveOrganizationID, filter)
		if positionErr != nil {
			return Page{}, positionErr
		}
		result.positions = append(result.positions, position)
	}
	if len(profilePage.Items) > 0 {
		result.Source += "+account_business_profile_audit"
	}
	if len(membershipPage.Items) > 0 {
		result.Source += "+organization_member_audit"
	}
	if len(usagePage.Items) > 0 {
		result.Source += "+ai_invocations"
	}
	if len(pointPage.Items) > 0 {
		result.Source += "+image_ai_point_debits"
	}
	if len(resourcePage.Items) > 0 {
		result.Source += "+member_resource_audit"
	}
	// Each source is fetched independently. The merged page can therefore be
	// truncated even when neither source returned its own page cursor; the
	// cursor still needs to carry the last emitted position from both streams
	// so the events beyond the merge boundary remain reachable.
	if mergedTruncated || history.Next != nil || resourcePage.Next != nil || profilePage.Next != nil || membershipPage.Next != nil || usagePage.Next != nil || pointPage.Next != nil {
		if len(result.positions) == 0 {
			return Page{}, registry.ErrUnavailable
		}
		result.NextCursor = &result.positions[len(result.positions)-1]
	}
	return result, nil
}
func encodeCursor(state cursorState, organization string, filter Filter) (string, error) {
	wire := positionWire{Organization: organization, Actor: filter.ActorSubject, Kind: string(filter.Kind), ResourceOperation: filter.ResourceOperation, ProfileOperation: filter.ProfileOperation, MembershipOperation: filter.MembershipOperation, Content: filter.Content, MemberID: filter.MemberID, Period: filter.Period, AsOf: filter.AsOf}
	if state.source != nil {
		wire.Source = &sourcePosition{Time: state.source.OccurredAt.UTC(), Account: state.source.AccountID, Version: strconv.FormatInt(state.source.Version, 10)}
	}
	if state.resources != nil {
		wire.Resources = &auditCursor{Time: state.resources.CreatedAt.UTC().Format(time.RFC3339Nano), Key: state.resources.Key}
	}
	if state.profile != nil {
		wire.Profile = &auditCursor{Time: state.profile.CreatedAt.UTC().Format(time.RFC3339Nano), Key: state.profile.Key}
	}
	if state.membership != nil {
		wire.Membership = &auditCursor{Time: state.membership.CreatedAt.UTC().Format(time.RFC3339Nano), Key: state.membership.Key}
	}
	if state.usage != nil {
		wire.Usage = &auditCursor{Time: state.usage.CreatedAt.UTC().Format(time.RFC3339Nano), Key: state.usage.Key}
	}
	if state.points != nil {
		wire.Points = &auditCursor{Time: state.points.CreatedAt.UTC().Format(time.RFC3339Nano), Key: state.points.EventID}
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return "", registry.ErrUnavailable
	}
	value := base64.RawURLEncoding.EncodeToString(data)
	if len(value) > 3072 {
		return "", registry.ErrUnavailable
	}
	return value, nil
}
func decodeCursor(value string) (positionWire, error) {
	if len(value) > 3072 {
		return positionWire{}, registry.ErrInvalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return positionWire{}, registry.ErrInvalid
	}
	var wire positionWire
	if json.Unmarshal(data, &wire) != nil {
		return positionWire{}, registry.ErrInvalid
	}
	canonical, err := json.Marshal(wire)
	if err != nil || base64.RawURLEncoding.EncodeToString(canonical) != value {
		return positionWire{}, registry.ErrInvalid
	}
	return wire, nil
}
func parseCursor(value, organization string, filter Filter) (cursorState, error) {
	if value == "" {
		return cursorState{}, nil
	}
	wire, err := decodeCursor(value)
	if err != nil || wire.Organization != organization || wire.Actor != filter.ActorSubject || wire.Kind != string(filter.Kind) || wire.ResourceOperation != filter.ResourceOperation || wire.ProfileOperation != filter.ProfileOperation || wire.MembershipOperation != filter.MembershipOperation || wire.Content != filter.Content || wire.MemberID != filter.MemberID || wire.Period != filter.Period || wire.AsOf != filter.AsOf || wire.Source == nil && wire.Resources == nil && wire.Profile == nil && wire.Membership == nil && wire.Usage == nil && wire.Points == nil {
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
	if wire.Resources != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Resources.Time)
		id, idErr := strconv.ParseInt(wire.Resources.Key, 10, 64)
		if err != nil || createdAt.IsZero() || idErr != nil || id < 1 {
			return cursorState{}, registry.ErrInvalid
		}
		state.resources = &AuditPosition{CreatedAt: createdAt, Key: wire.Resources.Key}
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
	if wire.Usage != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Usage.Time)
		if err != nil || createdAt.IsZero() || wire.Usage.Key == "" {
			return cursorState{}, registry.ErrInvalid
		}
		state.usage = &AuditPosition{CreatedAt: createdAt, Key: wire.Usage.Key}
	}
	if wire.Points != nil {
		createdAt, err := time.Parse(time.RFC3339Nano, wire.Points.Time)
		if err != nil || createdAt.IsZero() || wire.Points.Key == "" || len(wire.Points.Key) > 128 {
			return cursorState{}, registry.ErrInvalid
		}
		state.points = &orgresource.ImagePointAuditPosition{CreatedAt: createdAt, EventID: wire.Points.Key}
	}
	// A canonical re-encoding rejects unknown/duplicate fields, alternate JSON
	// spellings, trailing data and noncanonical encodings without retaining input.
	return state, nil
}

func validResourceEvent(item AdditionalAuditEvent) bool {
	if item.Resource == nil {
		return false
	}
	quantity, err := strconv.ParseInt(item.Resource.Quantity, 10, 64)
	if err != nil || strconv.FormatInt(quantity, 10) != item.Resource.Quantity || quantity < 0 {
		return false
	}
	if item.EventType == "account_member_ai_point_limit.changed" {
		return item.ObjectType == "member_ai_point_limit" && item.Operation == "set_member_ai_point_limit" && item.Resource.Type == orgresource.ResourceAIPoint
	}
	return item.EventType == "account_member_resource.changed" && item.ObjectType == "member_resource" && (item.Operation == "allocate_member_resource" || item.Operation == "reclaim_member_resource") && (item.Resource.Type == orgresource.ResourceStoreRenewalPeriod || item.Resource.Type == orgresource.ResourceDataRow) && quantity > 0
}
