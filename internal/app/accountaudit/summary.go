package accountaudit

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"time"

	"task-processor/internal/authidentity"
	registry "task-processor/internal/sourceaccountregistry"
)

var ErrSummaryNotConfigured = errors.New("account audit summary sources not configured")

type SummaryCounts struct {
	Operations  string `json:"operations"`
	Members     string `json:"members"`
	Permissions string `json:"permissions"`
	Resources   string `json:"resources"`
}
type SummaryWindow struct {
	From time.Time `json:"from"`
	AsOf time.Time `json:"asOf"`
}
type Summary struct {
	SchemaVersion           string        `json:"schemaVersion"`
	UserID                  string        `json:"userId"`
	EffectiveOrganizationID string        `json:"effectiveOrganizationId"`
	Coverage                string        `json:"coverage"`
	Window                  SummaryWindow `json:"window"`
	Counts                  SummaryCounts `json:"counts"`
}

func summaryMissing(source any) bool {
	if source == nil {
		return true
	}
	value := reflect.ValueOf(source)
	return value.Kind() == reflect.Ptr && value.IsNil()
}

// ReadSummary consumes the same committed events as the unfiltered list.
// No count is returned unless the entire window has been read successfully.
func (q *Query) ReadSummary(ctx context.Context) (Summary, error) {
	return q.readSummary(ctx, time.Now().UTC().Truncate(time.Microsecond))
}
func (q *Query) readSummary(ctx context.Context, asOf time.Time) (Summary, error) {
	if ctx == nil {
		return Summary{}, registry.ErrAuthenticationRequired
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return Summary{}, registry.ErrAuthenticationRequired
	}
	if identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID {
		return Summary{}, registry.ErrForbidden
	}
	if q == nil || summaryMissing(q.history) || summaryMissing(q.profile) || summaryMissing(q.membership) || summaryMissing(q.resources) || summaryMissing(q.points) {
		return Summary{}, ErrSummaryNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, registry.Timeout)
	defer cancel()
	query := *q
	// Observed usage can include unsuccessful invocations. It is not a success
	// receipt, and its source is not required for committed-operation counts.
	query.usage = nil
	from := asOf.Add(-30 * 24 * time.Hour)
	seen := map[string]Event{}
	cursors := map[string]bool{}
	cursor := ""
	var operations, members, permissions, resources int64
	for {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		page, err := query.Read(ctx, registry.MaxPageLimit, cursor)
		if err != nil {
			return Summary{}, err
		}
		if err = ctx.Err(); err != nil {
			return Summary{}, err
		}
		pastWindow := false
		for _, event := range page.Items {
			if event.Time.Before(from) {
				pastWindow = true
				break
			}
			if !event.Time.Before(asOf) {
				continue
			}
			key, member, permission, resource, err := summaryIdentity(event)
			if err != nil {
				return Summary{}, err
			}
			if previous, exists := seen[key]; exists {
				if !reflect.DeepEqual(previous, event) {
					return Summary{}, registry.ErrUnavailable
				}
				continue
			}
			seen[key] = event
			if operations == math.MaxInt64 {
				return Summary{}, registry.ErrUnavailable
			}
			operations++
			if member {
				members++
			}
			if permission {
				permissions++
			}
			if resource {
				resources++
			}
		}
		if pastWindow || page.NextCursor == nil {
			break
		}
		if len(page.Items) == 0 || *page.NextCursor == cursor || cursors[*page.NextCursor] {
			return Summary{}, registry.ErrUnavailable
		}
		cursor = *page.NextCursor
		cursors[cursor] = true
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return Summary{}, registry.ErrAuthenticationRequired
	}
	return Summary{
		SchemaVersion: "account-audit-summary-v1", UserID: identity.UserID, EffectiveOrganizationID: identity.EffectiveOrganizationID,
		Coverage: "current_account_audit_committed_events",
		Window:   SummaryWindow{From: from, AsOf: asOf},
		Counts:   SummaryCounts{Operations: strconv.FormatInt(operations, 10), Members: strconv.FormatInt(members, 10), Permissions: strconv.FormatInt(permissions, 10), Resources: strconv.FormatInt(resources, 10)},
	}, nil
}

// Operation identities vary by owner; revision or actor must not invent a
// second Resource operation. The configured membership reader binds project.
func summaryIdentity(event Event) (string, bool, bool, bool, error) {
	if event.Result != "succeeded" {
		return "", false, false, false, registry.ErrUnavailable
	}
	parts := []string{event.Relation.Type, event.Relation.Reference}
	member, permission, resource := false, false, false
	switch event.EventType {
	case "source_account.operation_committed", "account_business_profile.updated":
		parts = append(parts, event.Relation.Version)
	case "organization_membership.changed":
		if event.Operation != "invite" && event.Operation != "role" && event.Operation != "remove" {
			return "", false, false, false, registry.ErrUnavailable
		}
		parts = append(parts, event.Actor)
		member, permission = true, event.Operation == "role"
	case "account_member_resource.changed", "account_member_ai_point_limit.changed", "account_ai_points.committed":
		resource = true
	default:
		return "", false, false, false, registry.ErrUnavailable
	}
	key, _ := json.Marshal(parts)
	return string(key), member, permission, resource, nil
}
