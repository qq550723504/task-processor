package httpapi

import (
	"context"
	"math"
	"strconv"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
)

func (s *supplyProductAgent) titleSelection(ctx context.Context, scope agent.Scope, selected agentconfig.TemplateRef) (aicapability.ModelProfile, agentconfig.OrganizationAgent, agentconfig.Template, error) {
	var profile aicapability.ModelProfile
	var config agentconfig.OrganizationAgent
	var template agentconfig.Template
	if s == nil || s.agent == nil || s.agent.configuration == nil || s.agent.selectTitleProfile == nil {
		return profile, config, template, record.ErrNotReady
	}
	revision, err := strconv.ParseUint(selected.Revision, 10, 63)
	if !agentconfig.UUID(selected.TemplateID) || err != nil || revision == 0 || strconv.FormatUint(revision, 10) != selected.Revision {
		return profile, config, template, preparation.ErrInvalid
	}
	config, err = s.agent.configuration.ReadAgent(ctx, scope, s.agent.definition.ID)
	if err != nil || config.Activation != "ENABLED" {
		return profile, config, template, record.ErrNotReady
	}
	template, err = s.agent.configuration.ReadTemplate(ctx, scope, s.agent.definition.ID, selected.TemplateID, revision)
	if err != nil || template.Version != selected.Revision || template.Lifecycle != "ACTIVE" || template.TargetPlatform != "shein" || template.DefaultKnowledgeBaseID != "" {
		return profile, config, template, record.ErrNotReady
	}
	profile, err = s.agent.selectTitleProfile(ctx, scope.OrganizationID)
	if err != nil || profile.Validate() != nil || profile.Currency != s.agent.config.Limits.Currency {
		return profile, config, template, record.ErrNotReady
	}
	return profile, config, template, nil
}
func (s *supplyProductAgent) titleQuote(scope agent.Scope, profile aicapability.ModelProfile, config agentconfig.OrganizationAgent, template agentconfig.Template) string {
	return collection.Digest(struct {
		Scope                                                                                      agent.Scope
		Profile                                                                                    aicapability.ModelProfile
		Limits                                                                                     agent.Limits
		AgentID, AgentVersion, Epoch, AgentRevision, TemplateID, TemplateVersion, TemplateRevision string
	}{scope, profile, s.agent.config.Limits, s.agent.definition.ID, s.agent.definition.Version, config.ActivationEpoch, config.Revision, template.TemplateID, template.Version, template.Revision})
}
func (s *supplyProductAgent) validateTitleSelection(ctx context.Context, scope agent.Scope, selected agentconfig.TemplateRef, quote string) (aicapability.ModelProfile, error) {
	profile, config, template, err := s.titleSelection(ctx, scope, selected)
	if err != nil {
		return profile, err
	}
	if s.titleQuote(scope, profile, config, template) != quote {
		return profile, record.ErrConflict
	}
	return profile, nil
}
func (s *supplyProductAgent) options(ctx context.Context, q collection.Query) (supplyapp.OptimizationOptions, error) {
	out := supplyapp.OptimizationOptions{Titles: []supplyapp.TitleOptimizationChoice{}, ImageReason: "当前企业没有已准入的供应链图片模板"}
	if s == nil || s.agent == nil {
		out.Reason = "商品标题智能体未配置"
		return out, nil
	}
	identity, err := s.agent.freshIdentity(ctx)
	if err != nil {
		out.Reason = "当前身份或企业未开放标题优化"
		return out, nil
	}
	scope := agent.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}
	config, err := s.agent.configuration.ReadAgent(ctx, scope, s.agent.definition.ID)
	if err != nil || config.Activation != "ENABLED" {
		out.Reason = "请先在我的智能体中启用商品标题智能体"
		return out, nil
	}
	templates, next, err := s.agent.configuration.Templates(ctx, scope, s.agent.definition.ID, q.After, "ACTIVE", q.Limit)
	if err != nil {
		return out, err
	}
	out.NextCursor = next
	for _, template := range templates {
		if template.TargetPlatform != "shein" || template.DefaultKnowledgeBaseID != "" {
			continue
		}
		selected := agentconfig.TemplateRef{TemplateID: template.TemplateID, Revision: template.Version}
		profile, config, current, err := s.titleSelection(ctx, scope, selected)
		if err != nil {
			continue
		}
		points, err := profile.PointTariff.Points(profile.MaximumPromptTokens, profile.MaximumCompletionTokens)
		calls := int64(s.agent.config.Limits.ModelCalls)
		if err != nil || calls < 1 || points > math.MaxInt64/calls {
			continue
		}
		out.Titles = append(out.Titles, supplyapp.TitleOptimizationChoice{AgentID: s.agent.definition.ID, TemplateID: current.TemplateID, Revision: current.Version, Name: current.Name, QuoteHash: s.titleQuote(scope, profile, config, current), MaximumCostMicros: s.agent.config.Limits.CostMicros, Currency: s.agent.config.Limits.Currency, MaximumPoints: points * calls, PriceVersion: profile.PointTariff.PriceVersion})
	}
	if len(out.Titles) == 0 {
		out.Reason = "没有可执行的 SHEIN 标题模板或模型报价；可直接补全并上传"
	}
	return out, nil
}
func (s *supplyProductAgent) authorizeRequest(ctx context.Context, in preparation.OperationInput) error {
	if in.Validate() != nil {
		return preparation.ErrInvalid
	}
	if in.ImageTemplateID != "" || in.TitleTemplateID == "" {
		return record.ErrNotReady
	}
	identity, err := s.agent.freshIdentity(ctx)
	if err != nil {
		return preparation.ErrForbidden
	}
	_, err = s.validateTitleSelection(ctx, agent.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}, agentconfig.TemplateRef{TemplateID: in.TitleTemplateID, Revision: in.TitleTemplateRevision}, in.TitleQuoteHash)
	return err
}
