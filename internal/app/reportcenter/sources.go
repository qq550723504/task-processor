// Package reportcenterapp projects existing authorized owners for historical reports.
package reportcenterapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	validator "task-processor/internal/marketplace/validator"
	"task-processor/internal/product/review"
	rc "task-processor/internal/reportcenter"
	"time"
)

type ReviewReader interface {
	Get(context.Context, string) (review.View, error)
}
type Sources struct {
	Reviews ReviewReader
	Records record.Reader
	Policy  authz.StaticAuthorizer
}

func (s *Sources) Read(ctx context.Context, scope rc.Scope, kind, id string) (rc.Snapshot, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.UserID != scope.ActorID || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || !identity.TokenExpiresAt.After(time.Now()) {
		return rc.Snapshot{}, rc.ErrForbidden
	}
	permission := authz.PermissionLocalAgentWrite
	if kind == "SHEIN_RECORD" {
		permission = authz.PermissionListingKitAdminRead
	}
	if !authz.AllowedOrganization(ctx, s.Policy, "", scope.OrganizationID, identity.Roles, permission) {
		return rc.Snapshot{}, rc.ErrForbidden
	}
	if kind == "TITLE_REVIEW" {
		if s.Reviews == nil {
			return rc.Snapshot{}, rc.ErrUnavailable
		}
		v, e := s.Reviews.Get(ctx, id)
		if e != nil {
			return rc.Snapshot{}, sourceError(e)
		}
		if v.ID != id || v.Owner != scope.ActorID {
			return rc.Snapshot{}, rc.ErrNotFound
		}
		if review.ValidateView(v) != nil {
			return rc.Snapshot{}, rc.ErrUnavailable
		}
		result := rc.Snapshot{SourceInfo: rc.SourceInfo{Ref: rc.SourceRef{Kind: kind, ID: id, Version: strconv.FormatUint(v.Revision, 10) + ":" + v.State}, Title: "标题审核 · " + v.Input.ProductKey, ProductKey: v.Input.ProductKey}, Content: rc.Document{SchemaVersion: 1}}
		fields := []rc.Field{{Label: "保存时状态", Value: v.State}, {Label: "原始标题", Value: v.Before}, {Label: "建议标题", Value: v.Title}, {Label: "商品基础版本", Value: strconv.FormatUint(v.Input.BaseVersion, 10)}, {Label: "审核策略", Value: v.Policy}}
		quality, _ := json.Marshal(v.Quality)
		fields = append(fields, rc.Field{Label: "质量评分", Value: string(quality)})
		if len(v.Unresolved) > 0 {
			fields = append(fields, rc.Field{Label: "未解决提示", Value: strings.Join(v.Unresolved, "\n")})
		}
		result.Content.Sections = append(result.Content.Sections, rc.Section{Title: "标题审核历史快照", Fields: fields})
		decisions := []rc.Field{}
		for _, d := range v.History {
			decisions = append(decisions, rc.Field{Label: fmt.Sprintf("%d · %s", d.Revision, d.Action), Value: d.At.UTC().Format(time.RFC3339Nano) + "\n操作人：" + d.Actor + "\n原标题：" + d.Before + "\n结果标题：" + d.After})
		}
		if len(decisions) > 0 {
			result.Content.Sections = append(result.Content.Sections, rc.Section{Title: "保存时审核记录", Fields: decisions})
		}
		if v.Receipt != nil {
			r := v.Receipt
			result.Content.Sections = append(result.Content.Sections, rc.Section{Title: "保存时 Apply 回执", Fields: []rc.Field{{Label: "商品版本", Value: strconv.FormatUint(r.ProductVersion, 10)}, {Label: "发布事实 ID", Value: r.PublicationID}, {Label: "应用人", Value: r.Actor}, {Label: "应用时间", Value: r.At.UTC().Format(time.RFC3339Nano)}}})
		}
		return result, nil
	}
	if kind != "SHEIN_RECORD" {
		return rc.Snapshot{}, rc.ErrInvalid
	}
	if s.Records == nil {
		return rc.Snapshot{}, rc.ErrUnavailable
	}
	r, e := s.Records.ReadOfflinePackage(ctx, listingtask.Actor{TenantID: scope.OrganizationID, UserID: scope.ActorID, Roles: identity.Roles}, id)
	if e != nil {
		return rc.Snapshot{}, sourceError(e)
	}
	if r.ID != id || r.OrganizationID != scope.OrganizationID || r.OwnerUserID != scope.ActorID {
		return rc.Snapshot{}, rc.ErrNotFound
	}
	var d validator.DiagnosticResult
	sum := sha256.Sum256(r.Diagnostic)
	if r.Input.Validate() != nil || hex.EncodeToString(sum[:]) != r.DiagnosticHash || len(r.Diagnostic) > record.MaxPayloadBytes || json.Unmarshal(r.Diagnostic, &d) != nil || !d.DiagnosticOnly || d.Target.Marketplace != "shein" || d.Target.Site != "" || d.Action != r.Input.Action || d.RuleVersion != r.RuleRevision || d.Input.BindingVersion != r.PolicyRevision || d.OfflineChecks.Status != r.DiagnosticStatus || validator.ValidateBoundInput(d.Input, "sha256:"+r.PackageHash) != nil || d.Freshness.Validate(d.Input.Digest, d.Input.EvaluatedAt) != nil {
		return rc.Snapshot{}, rc.ErrUnavailable
	}
	result := rc.Snapshot{SourceInfo: rc.SourceInfo{Ref: rc.SourceRef{Kind: kind, ID: id, Version: r.InputHash}, Title: "SHEIN 商品资料与离线诊断 · " + r.Input.ProductKey, ProductKey: r.Input.ProductKey, StoreID: r.Input.StoreID, SourceAt: &r.CreatedAt}, Content: rc.Document{SchemaVersion: 1}}
	result.Content.Sections = []rc.Section{{Title: "保存时商品资料", Fields: []rc.Field{{Label: "商品版本", Value: strconv.FormatUint(r.Input.SnapshotVersion, 10)}, {Label: "店铺 ID", Value: r.Input.StoreID}, {Label: "国家与语言", Value: r.Input.Country + " / " + r.Input.Language}, {Label: "原操作", Value: string(r.Input.Action)}, {Label: "资料摘要 SHA-256", Value: r.PackageHash}, {Label: "诊断摘要 SHA-256", Value: r.DiagnosticHash}, {Label: "规则版本", Value: r.RuleRevision}, {Label: "策略版本", Value: r.PolicyRevision}}}, {Title: "保存时离线诊断（不代表平台发布许可）", Fields: []rc.Field{{Label: "离线状态", Value: string(d.OfflineChecks.Status)}, {Label: "诊断范围", Value: d.Scope}, {Label: "外部新鲜度", Value: string(d.Freshness.Status)}, {Label: "评估时间", Value: d.Input.EvaluatedAt.UTC().Format(time.RFC3339Nano)}}}}
	for _, c := range d.OfflineChecks.Checks {
		value := string(c.Status)
		if c.Message != "" {
			value += " · " + c.Message
		}
		if c.Guidance != "" {
			value += "\n" + c.Guidance
		}
		result.Content.Sections[1].Fields = append(result.Content.Sections[1].Fields, rc.Field{Label: c.Code, Value: value})
	}
	keys := append([]string(nil), d.NotEvaluated...)
	sort.Strings(keys)
	if len(keys) > 0 {
		fields := []rc.Field{}
		for _, key := range keys {
			value := d.NotEvaluatedReasons[key]
			if value == "" {
				value = "未评估"
			}
			fields = append(fields, rc.Field{Label: key, Value: value})
		}
		result.Content.Sections = append(result.Content.Sections, rc.Section{Title: "未评估的外部条件", Fields: fields})
	}
	return result, nil
}
func sourceError(e error) error {
	switch {
	case errors.Is(e, review.ErrForbidden) || errors.Is(e, record.ErrForbidden):
		return rc.ErrForbidden
	case errors.Is(e, review.ErrNotFound) || errors.Is(e, record.ErrNotFound):
		return rc.ErrNotFound
	default:
		return rc.ErrUnavailable
	}
}
