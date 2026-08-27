package httpapi

import (
	"strings"
	"time"

	"task-processor/internal/imageagent"
)

type slotDTO struct {
	ID                string                `json:"id"`
	Role              imageagent.SlotRole   `json:"role"`
	SourceAssetIDs    []string              `json:"source_asset_ids"`
	StyleReferenceIDs []string              `json:"style_reference_ids,omitempty"`
	Brief             string                `json:"brief,omitempty"`
	IdempotencyKey    string                `json:"idempotency_key"`
	Status            imageagent.SlotStatus `json:"status"`
}

func (d slotDTO) domain() imageagent.Slot {
	return imageagent.Slot{
		ID: d.ID, Role: d.Role, SourceAssetIDs: append([]string(nil), d.SourceAssetIDs...),
		StyleReferenceIDs: append([]string(nil), d.StyleReferenceIDs...), Brief: d.Brief,
		IdempotencyKey: d.IdempotencyKey, Status: d.Status,
	}
}

func newSlotDTO(slot imageagent.Slot) slotDTO {
	return slotDTO{
		ID: slot.ID, Role: slot.Role, SourceAssetIDs: append([]string(nil), slot.SourceAssetIDs...),
		StyleReferenceIDs: append([]string(nil), slot.StyleReferenceIDs...), Brief: slot.Brief,
		IdempotencyKey: slot.IdempotencyKey, Status: slot.Status,
	}
}

type planDTO struct {
	Revision          int64     `json:"revision"`
	ParentRevision    int64     `json:"parent_revision"`
	IdempotencyKey    string    `json:"idempotency_key"`
	SourceAssetIDs    []string  `json:"source_asset_ids"`
	StyleReferenceIDs []string  `json:"style_reference_ids,omitempty"`
	Slots             []slotDTO `json:"slots"`
	CreatedBy         string    `json:"created_by,omitempty"`
}

func (d planDTO) domain() imageagent.Plan {
	plan := imageagent.Plan{
		Revision: d.Revision, ParentRevision: d.ParentRevision, IdempotencyKey: d.IdempotencyKey,
		SourceAssetIDs:    append([]string(nil), d.SourceAssetIDs...),
		StyleReferenceIDs: append([]string(nil), d.StyleReferenceIDs...), CreatedBy: d.CreatedBy,
		Slots: make([]imageagent.Slot, len(d.Slots)),
	}
	for index, slot := range d.Slots {
		plan.Slots[index] = slot.domain()
	}
	return plan
}

func newPlanDTO(plan imageagent.Plan) planDTO {
	dto := planDTO{
		Revision: plan.Revision, ParentRevision: plan.ParentRevision, IdempotencyKey: plan.IdempotencyKey,
		SourceAssetIDs:    append([]string(nil), plan.SourceAssetIDs...),
		StyleReferenceIDs: append([]string(nil), plan.StyleReferenceIDs...), CreatedBy: plan.CreatedBy,
		Slots: make([]slotDTO, len(plan.Slots)),
	}
	for index, slot := range plan.Slots {
		dto.Slots[index] = newSlotDTO(slot)
	}
	return dto
}

type budgetDTO struct {
	MaxImages                int           `json:"max_images"`
	MaxAgentSteps            int           `json:"max_agent_steps"`
	MaxModelCalls            int           `json:"max_model_calls"`
	MaxRepairAttemptsPerSlot int           `json:"max_repair_attempts_per_slot"`
	MaxCostMicros            int64         `json:"max_cost_micros"`
	MaxElapsed               time.Duration `json:"max_elapsed"`
}

func (d budgetDTO) domain() imageagent.Budget {
	return imageagent.Budget{
		MaxImages: d.MaxImages, MaxAgentSteps: d.MaxAgentSteps, MaxModelCalls: d.MaxModelCalls,
		MaxRepairAttemptsPerSlot: d.MaxRepairAttemptsPerSlot, MaxCostMicros: d.MaxCostMicros, MaxElapsed: d.MaxElapsed,
	}
}

func newBudgetDTO(value imageagent.Budget) budgetDTO {
	return budgetDTO{
		MaxImages: value.MaxImages, MaxAgentSteps: value.MaxAgentSteps, MaxModelCalls: value.MaxModelCalls,
		MaxRepairAttemptsPerSlot: value.MaxRepairAttemptsPerSlot, MaxCostMicros: value.MaxCostMicros, MaxElapsed: value.MaxElapsed,
	}
}

type budgetUsageDTO struct {
	Images              int           `json:"images"`
	AgentSteps          int           `json:"agent_steps"`
	ModelCalls          int           `json:"model_calls"`
	EstimatedCostMicros int64         `json:"estimated_cost_micros"`
	Elapsed             time.Duration `json:"elapsed"`
}

func newBudgetUsageDTO(value imageagent.BudgetUsage) budgetUsageDTO {
	return budgetUsageDTO{
		Images: value.Images, AgentSteps: value.AgentSteps, ModelCalls: value.ModelCalls,
		EstimatedCostMicros: value.EstimatedCostMicros, Elapsed: value.Elapsed,
	}
}

type blockDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SlotID  string `json:"slot_id,omitempty"`
}

func newBlockDTO(value *imageagent.Block) *blockDTO {
	if value == nil {
		return nil
	}
	return &blockDTO{Code: value.Code, Message: value.Message, SlotID: value.SlotID}
}

type runDTO struct {
	ID                 string               `json:"id"`
	BusinessTaskID     string               `json:"business_task_id,omitempty"`
	TenantID           string               `json:"tenant_id"`
	UserID             string               `json:"user_id"`
	Mode               imageagent.RunMode   `json:"mode"`
	IdempotencyKey     string               `json:"idempotency_key"`
	Status             imageagent.RunStatus `json:"status"`
	CurrentNode        string               `json:"current_node"`
	ActivePlanRevision int64                `json:"active_plan_revision"`
	Version            int64                `json:"version"`
	Budget             budgetDTO            `json:"budget"`
	Usage              budgetUsageDTO       `json:"usage"`
	Block              *blockDTO            `json:"block,omitempty"`
}

func newRunDTO(run imageagent.Run) runDTO {
	return runDTO{
		ID: run.ID, BusinessTaskID: run.BusinessTaskID, TenantID: run.TenantID, UserID: run.UserID,
		Mode: run.Mode, IdempotencyKey: run.IdempotencyKey, Status: run.Status, CurrentNode: run.CurrentNode,
		ActivePlanRevision: run.ActivePlanRevision, Version: run.Version, Budget: newBudgetDTO(run.Budget),
		Usage: newBudgetUsageDTO(run.Usage), Block: newBlockDTO(run.Block),
	}
}

type assetCandidateDTO struct {
	AssetID       string            `json:"asset_id"`
	URL           string            `json:"url"`
	SourceAssetID string            `json:"source_asset_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func newAssetCandidateDTO(value imageagent.AssetCandidate) assetCandidateDTO {
	metadata := make(map[string]string, len(value.Metadata))
	for key, item := range value.Metadata {
		metadata[key] = item
	}
	return assetCandidateDTO{AssetID: value.AssetID, URL: value.URL, SourceAssetID: value.SourceAssetID, Metadata: metadata}
}

type slotProjectionDTO struct {
	Slot       slotDTO             `json:"slot"`
	Attempt    int                 `json:"attempt"`
	Candidates []assetCandidateDTO `json:"candidates"`
	ErrorCode  string              `json:"error_code,omitempty"`
}

func newSlotProjectionDTO(value imageagent.SlotProjection) slotProjectionDTO {
	dto := slotProjectionDTO{Slot: newSlotDTO(value.Slot), Attempt: value.Attempt, ErrorCode: value.ErrorCode, Candidates: make([]assetCandidateDTO, 0, len(value.Candidates))}
	for _, candidate := range value.Candidates {
		url, err := imageagent.ValidateSafeImageURL(candidate.URL)
		if err != nil {
			continue
		}
		candidate.URL = url
		dto.Candidates = append(dto.Candidates, newAssetCandidateDTO(candidate))
	}
	return dto
}

type runProjectionResponse struct {
	Run               runDTO               `json:"run"`
	Plan              planDTO              `json:"plan"`
	Slots             []slotProjectionDTO  `json:"slots"`
	ResultDigest      string               `json:"result_digest,omitempty"`
	Actions           []imageagent.Action  `json:"actions"`
	LastEventID       int64                `json:"last_event_id"`
	ProjectionVersion int64                `json:"projection_version"`
	AssetCatalog      []authorizedAssetDTO `json:"asset_catalog"`
	PendingCommand    *pendingCommandDTO   `json:"pending_command,omitempty"`
	CommandIngress    commandIngressDTO    `json:"command_ingress"`
}

type authorizedAssetDTO struct {
	ID         string                         `json:"id"`
	Type       imageagent.AuthorizedAssetType `json:"type"`
	DisplayURL string                         `json:"display_url,omitempty"`
	Label      string                         `json:"label,omitempty"`
}

type pendingCommandDTO struct {
	ActionID        string     `json:"action_id"`
	Kind            string     `json:"kind"`
	Phase           string     `json:"phase"`
	Status          string     `json:"status"`
	PlanRevision    int64      `json:"plan_revision"`
	SlotID          string     `json:"slot_id,omitempty"`
	FailureCode     string     `json:"failure_code,omitempty"`
	FailureCategory string     `json:"failure_category,omitempty"`
	FailureMessage  string     `json:"failure_message,omitempty"`
	LastFailedAt    *time.Time `json:"last_failed_at,omitempty"`
	Attempt         int        `json:"attempt,omitempty"`
}

type commandIngressDTO struct {
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
	Exhausted bool   `json:"exhausted"`
	Reason    string `json:"reason,omitempty"`
}

func newRunProjectionResponse(value imageagent.RunProjection) runProjectionResponse {
	response := runProjectionResponse{
		Run: newRunDTO(value.Run), Plan: newPlanDTO(value.Plan), ResultDigest: value.ResultDigest,
		Actions: append([]imageagent.Action(nil), value.Actions...), LastEventID: value.LastEventID,
		ProjectionVersion: value.ProjectionVersion,
		Slots:             make([]slotProjectionDTO, len(value.Slots)),
		CommandIngress:    commandIngressDTO{Used: value.CommandIngress.Used, Limit: value.CommandIngress.Limit, Exhausted: value.CommandIngress.Exhausted, Reason: value.CommandIngress.Reason},
	}
	response.AssetCatalog = make([]authorizedAssetDTO, 0, len(value.AssetCatalog.Assets))
	for _, asset := range value.AssetCatalog.Assets {
		if strings.TrimSpace(asset.ID) == "" || (asset.Type != imageagent.AuthorizedAssetSource && asset.Type != imageagent.AuthorizedAssetStyle) {
			continue
		}
		displayURL := ""
		if strings.TrimSpace(asset.DisplayURL) != "" {
			if safeURL, err := imageagent.ValidateSafeImageURL(asset.DisplayURL); err == nil {
				displayURL = safeURL
			}
		}
		response.AssetCatalog = append(response.AssetCatalog, authorizedAssetDTO{ID: strings.TrimSpace(asset.ID), Type: asset.Type, DisplayURL: displayURL, Label: strings.TrimSpace(asset.Label)})
	}
	if value.PendingCommand != nil {
		receipt := value.PendingCommand
		response.PendingCommand = &pendingCommandDTO{ActionID: receipt.ActionID, Kind: receipt.Kind, Phase: receipt.Phase, Status: receipt.Status, PlanRevision: receipt.PlanRevision, SlotID: receipt.SlotID, FailureCode: receipt.FailureCode, FailureCategory: receipt.FailureCategory, FailureMessage: receipt.FailureMessage, LastFailedAt: receipt.LastFailedAt, Attempt: receipt.Attempt}
	}
	for index, slot := range value.Slots {
		response.Slots[index] = newSlotProjectionDTO(slot)
	}
	return response
}
