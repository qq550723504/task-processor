package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	kernelmodule "task-processor/internal/kernel/module"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
)

const acquisitionImageSetBase = productAcquisitionBase + "/:operation_id/images"
const supplyImageSetBase = "/api/v1/workbench/supply-preparations/sources/:source_id/images"

type fullImageModule struct {
	application *fullImageApplication
	bind        func(context.Context, string) (context.Context, error)
}

func (fullImageModule) Name() string                { return "product-image-set" }
func (fullImageModule) Enabled(*config.Config) bool { return true }
func (m fullImageModule) Register(reg *kernelmodule.Registry) error {
	if m.application == nil || m.bind == nil {
		return imageagent.ErrCommandBlocked
	}
	reg.AddRoutes(m.routes()...)
	return nil
}

type fullImagePrepareBody struct {
	Template                *agentconfig.TemplateRef                     `json:"template,omitempty"`
	Target                  imageagent.ImageTargetSelection              `json:"target"`
	SharedOriginalIDs       []string                                     `json:"sharedOriginalIds,omitempty"`
	CarouselOriginalIDs     []string                                     `json:"carouselOriginalIds,omitempty"`
	DetailOriginalIDs       []string                                     `json:"detailOriginalIds,omitempty"`
	SelectedTaskIDs         []string                                     `json:"selectedTaskIds,omitempty"`
	OfficialPlacements      map[string]imageagent.OfficialImagePlacement `json:"officialPlacements,omitempty"`
	EffectiveCatalogVersion uint64                                       `json:"effectiveCatalogVersion,omitempty"`
	ApplyReceiptID          string                                       `json:"applyReceiptId,omitempty"`
}
type fullImageSelectionBody struct {
	ActionID        string                   `json:"actionId"`
	PlanRevision    int64                    `json:"planRevision"`
	ResultDigest    string                   `json:"resultDigest"`
	ExpectedHead    asset.ImageInventoryHead `json:"expectedHead"`
	Choices         []asset.ImageSetChoice   `json:"choices"`
	SelectionDigest string                   `json:"selectionDigest,omitempty"`
}

func (m fullImageModule) routes() []httproute.Descriptor {
	routes := []httproute.Descriptor{}
	for _, owner := range []struct {
		base, param string
		kind        imageagent.ImageSourceContextKind
	}{{acquisitionImageSetBase, "operation_id", imageagent.ImageSourceAcquisition}, {supplyImageSetBase, "source_id", imageagent.ImageSourceSupply}} {
		for _, action := range []struct{ method, suffix, name string }{
			{http.MethodGet, "/sources", "sources"}, {http.MethodPost, "/prepare", "prepare"},
			{http.MethodPost, "/requirements", "requirements"},
			{http.MethodGet, "/runs", "recent"},
			{http.MethodGet, "/by-key/:request_id", "verify-prepare"},
			{http.MethodGet, "/runs/:run_id", "read"}, {http.MethodGet, "/runs/:run_id/inventory", "inventory"},
			{http.MethodGet, "/runs/:run_id/approvals/:approval_id", "approval"},
			{http.MethodPost, "/runs/:run_id/confirm", "confirm"}, {http.MethodPost, "/runs/:run_id/regenerate", "regenerate"},
			{http.MethodPost, "/runs/:run_id/restart", "restart"},
			{http.MethodPost, "/runs/:run_id/preview", "preview"}, {http.MethodPost, "/runs/:run_id/approve", "approve"},
			{http.MethodPost, "/runs/:run_id/cancel", "cancel"}, {http.MethodPost, "/runs/:run_id/recover", "recover"}, {http.MethodPost, "/runs/:run_id/resume", "resume"},
		} {
			owner, action := owner, action
			permission := authz.PermissionImageAgentWrite
			if action.method == http.MethodGet {
				permission = authz.PermissionImageAgentRead
			}
			routes = append(routes, httproute.Descriptor{Method: action.method, Path: owner.base + action.suffix, Module: m.Name(), Permission: permission, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 30 * time.Second, Handler: func(c *gin.Context) { m.handle(c, owner.kind, c.Param(owner.param), action.name) }})
		}
	}
	return routes
}

func validateFullImageRun(identity authidentity.AuthenticatedIdentity, kind imageagent.ImageSourceContextKind, contextID, runID string, p imageagent.RunProjection) error {
	if p.Plan.Set == nil || p.Run.ID != runID || p.Run.ScopeProtocol != imageagent.OrganizationScopeProtocol || p.Run.TenantID != identity.TenantID || identity.EffectiveOrganizationID != identity.TenantID || p.Run.UserID != identity.UserID || p.Run.MemberID != identity.EffectiveMemberID || p.Run.BusinessTaskID != contextID || p.Plan.Set.Source.ContextKind != kind || p.Plan.Set.Source.OperationID != contextID {
		return imageagent.ErrRunNotFound
	}
	return nil
}

func (m fullImageModule) handle(c *gin.Context, kind imageagent.ImageSourceContextKind, contextID, action string) {
	a := m.application
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || identity.TenantID == "" || identity.TenantID != identity.EffectiveOrganizationID || identity.EffectiveMemberID == "" || identity.UserID == "" {
		writeFullImageError(c, imageagent.ErrIdentityRequired, false)
		return
	}
	if a == nil || a.service == nil || m.bind == nil {
		writeFullImageError(c, imageagent.ErrCommandBlocked, false)
		return
	}
	if !acquisitionHTTPUUID(contextID) || action != "sources" && action != "recent" && c.Request.URL.RawQuery != "" {
		writeFullImageError(c, imageagent.ErrValidation, false)
		return
	}
	ctx, err := m.bind(c.Request.Context(), c.GetHeader("Authorization"))
	if err != nil {
		writeFullImageError(c, err, false)
		return
	}
	exec := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: identity.EffectiveMemberID, BusinessTaskID: contextID}
	if action == "requirements" {
		var body fullImagePrepareBody
		if err = readFullImageJSON(c.Request, &body); err != nil {
			writeFullImageError(c, err, false)
			return
		}
		input := imageagent.PrepareImageSetInput{ContextKind: kind, ContextID: contextID, Target: body.Target, EffectiveCatalogVersion: body.EffectiveCatalogVersion, ApplyReceiptID: body.ApplyReceiptID}
		source, e := a.readSources.ReadImageSetSource(ctx, exec, input)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		if body.Target.Platform == "product" {
			c.JSON(http.StatusOK, gin.H{"groups": []gin.H{}, "platform": "product", "site": "", "categoryId": 0, "version": "", "nativeWidth": 1024, "nativeHeight": 1024})
			return
		}
		target, requirements, e := a.rules.ReadImageSetRequirements(ctx, exec, body.Target, source.Source)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		groups := make([]gin.H, 0, len(requirements.Groups))
		for _, group := range requirements.Groups {
			types := make([]gin.H, 0, len(group.Types))
			for _, t := range group.Types {
				types = append(types, gin.H{"type": t.Type, "minimum": t.Minimum, "maximum": t.Maximum, "nativeCompatible": goods.OfficialImageSizeAllowed(group.Group, t.Type, 1024, 1024)})
			}
			groups = append(groups, gin.H{"group": group.Group, "skc": group.SKC, "sku": group.SKU, "types": types})
		}
		c.JSON(http.StatusOK, gin.H{"groups": groups, "platform": target.Platform, "site": target.Site, "categoryId": target.CategoryID, "version": requirements.Version, "nativeWidth": 1024, "nativeHeight": 1024})
		return
	}
	if action == "verify-prepare" {
		if !emptyAcquisitionImageBody(c) {
			return
		}
		p, e := a.service.GetPreparedImageSet(ctx, kind, contextID, c.Param("request_id"))
		if e == nil {
			e = validateFullImageRun(identity, kind, contextID, p.Run.ID, p)
		}
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		source := p.Plan.Set.Source
		_, e = a.readSources.ReadImageSetSource(ctx, exec, imageagent.PrepareImageSetInput{ContextKind: kind, ContextID: contextID, Target: imageagent.ImageTargetSelection{Platform: p.Plan.Set.Target.Platform}, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID})
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		response, e := a.response(ctx, p)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		c.JSON(http.StatusOK, response)
		return
	}
	if action == "recent" {
		if !emptyAcquisitionImageBody(c) {
			return
		}
		query := c.Request.URL.Query()
		for key, values := range query {
			if key != "cursor" || len(values) != 1 {
				writeFullImageError(c, imageagent.ErrValidation, false)
				return
			}
		}
		items, next, e := a.readRecent(ctx, contextID, query.Get("cursor"), 20)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		visible := make([]imageagent.ImageSetRunSummary, 0, len(items))
		for _, item := range items {
			if item.ContextKind == kind {
				visible = append(visible, item)
			}
		}
		c.JSON(http.StatusOK, gin.H{"items": visible, "nextCursor": next})
		return
	}
	if action == "sources" {
		if !emptyAcquisitionImageBody(c) {
			return
		}
		query := c.Request.URL.Query()
		for key, values := range query {
			if (key != "effectiveCatalogVersion" && key != "applyReceiptId") || len(values) != 1 {
				writeFullImageError(c, imageagent.ErrValidation, false)
				return
			}
		}
		version := uint64(0)
		if raw := query.Get("effectiveCatalogVersion"); raw != "" {
			version, err = strconv.ParseUint(raw, 10, 63)
			if err != nil || version == 0 {
				writeFullImageError(c, imageagent.ErrValidation, false)
				return
			}
		}
		apply := query.Get("applyReceiptId")
		if apply != "" && (!acquisitionHTTPUUID(apply) || version == 0) {
			writeFullImageError(c, imageagent.ErrValidation, false)
			return
		}
		source, e := a.readSources.ReadImageSetSource(ctx, exec, imageagent.PrepareImageSetInput{ContextKind: kind, ContextID: contextID, Target: imageagent.ImageTargetSelection{Platform: "product"}, EffectiveCatalogVersion: version, ApplyReceiptID: apply})
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		originals := make([]gin.H, 0, len(source.Catalog.Assets))
		for _, original := range source.Catalog.Assets {
			originals = append(originals, gin.H{"id": original.ID, "displayUrl": original.DisplayURL, "width": original.Width, "height": original.Height})
		}
		evidence := source.Evidence
		if evidence == nil {
			evidence = map[string]string{}
		}
		c.JSON(http.StatusOK, gin.H{"contextKind": kind, "contextId": contextID, "source": source.Source, "originals": originals, "evidence": evidence, "manualReplacementAvailable": a.manualAvailable})
		return
	}
	if action == "prepare" || action == "regenerate" {
		var body fullImagePrepareBody
		if err = readFullImageJSON(c.Request, &body); err != nil {
			writeFullImageError(c, err, false)
			return
		}
		keys := c.Request.Header.Values("Idempotency-Key")
		if len(keys) != 1 || !acquisitionHTTPUUID(keys[0]) {
			writeFullImageError(c, imageagent.ErrValidation, false)
			return
		}
		input := imageagent.PrepareImageSetInput{ContextKind: kind, RequestID: keys[0], ContextID: contextID, Template: body.Template, Target: body.Target, SharedOriginalIDs: body.SharedOriginalIDs, CarouselOriginalIDs: body.CarouselOriginalIDs, DetailOriginalIDs: body.DetailOriginalIDs, SelectedTaskIDs: body.SelectedTaskIDs, OfficialPlacements: body.OfficialPlacements, EffectiveCatalogVersion: body.EffectiveCatalogVersion, ApplyReceiptID: body.ApplyReceiptID}
		if action == "regenerate" {
			p, e := a.service.Get(ctx, c.Param("run_id"))
			if e == nil {
				e = validateFullImageRun(identity, kind, contextID, c.Param("run_id"), p)
			}
			if e != nil {
				writeFullImageError(c, e, false)
				return
			}
			input.RegenerateFromRunID = p.Run.ID
		}
		prepared, e := a.service.PrepareImageSet(ctx, input)
		if e != nil {
			writeFullImageError(c, e, true)
			return
		}
		response, e := a.response(ctx, prepared.Projection)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		c.JSON(http.StatusCreated, response)
		return
	}
	runID := c.Param("run_id")
	if !acquisitionHTTPUUID(runID) {
		writeFullImageError(c, imageagent.ErrValidation, false)
		return
	}
	p, err := a.service.Get(ctx, runID)
	if err == nil {
		err = validateFullImageRun(identity, kind, contextID, runID, p)
	}
	if err != nil {
		writeFullImageError(c, err, false)
		return
	}
	// This is an owner read, never an input download or a provider call.
	source := p.Plan.Set.Source
	liveSource, err := a.readSources.ReadImageSetSource(ctx, exec, imageagent.PrepareImageSetInput{ContextKind: kind, ContextID: contextID, Target: imageagent.ImageTargetSelection{Platform: p.Plan.Set.Target.Platform}, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID})
	if err != nil {
		writeFullImageError(c, err, false)
		return
	}
	liveBinding, bound := liveSource.Source, source
	liveBinding.CatalogHash, bound.CatalogHash = "", ""
	if liveBinding != bound {
		writeFullImageError(c, imageagent.ErrRevisionConflict, false)
		return
	}
	switch action {
	case "approval":
		if !emptyAcquisitionImageBody(c) {
			return
		}
		actionID := c.Param("approval_id")
		if !acquisitionHTTPUUID(actionID) {
			writeFullImageError(c, imageagent.ErrValidation, false)
			return
		}
		commit, e := a.approvals.ReadApprovalCommit(ctx, identity.TenantID, actionID)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		if commit.ImageSet == nil || commit.ActionID != actionID || commit.TenantID != identity.TenantID || commit.ProductKey != source.ProductID || commit.TargetPlatform != p.Plan.Set.Target.Platform || commit.SourceSnapshotVersion != source.EffectiveVersion {
			writeFullImageError(c, asset.ErrApprovalConflict, false)
			return
		}
		selected := commit.ImageSet.Source
		if selected.ContextKind != string(kind) || selected.ItemID != contextID || selected.OriginalPublicationID != source.OriginalPublicationID || selected.OriginalSnapshotVersion != source.OriginalVersion || selected.EffectiveCatalogVersion != source.EffectiveVersion || selected.ApplyReceiptID != source.ApplyReceiptID {
			writeFullImageError(c, asset.ErrApprovalConflict, false)
			return
		}
		c.JSON(http.StatusOK, gin.H{"actionId": actionID, "selectionDigest": commit.ImageSet.Digest, "assets": commit.Assets})
		return
	case "read":
		if !emptyAcquisitionImageBody(c) {
			return
		}
		response, e := a.response(ctx, p)
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		c.JSON(http.StatusOK, response)
		return
	case "inventory":
		if !emptyAcquisitionImageBody(c) {
			return
		}
		inventory, e := a.inventories.ReadImageSetInventory(ctx, asset.InventoryScope{TenantID: identity.TenantID, ProductKey: source.ProductID, TargetPlatform: p.Plan.Set.Target.Platform, SourceSnapshotVersion: source.EffectiveVersion})
		if e != nil {
			writeFullImageError(c, e, false)
			return
		}
		var generic *asset.ImageSetInventory
		if p.Plan.Set.Target.Platform != "product" {
			value, e := a.inventories.ReadImageSetInventory(ctx, asset.InventoryScope{TenantID: identity.TenantID, ProductKey: source.ProductID, TargetPlatform: "product", SourceSnapshotVersion: source.EffectiveVersion})
			if e != nil {
				writeFullImageError(c, e, false)
				return
			}
			generic = &value
		}
		c.JSON(http.StatusOK, gin.H{"target": inventory, "generic": generic})
		return
	case "confirm":
		var body struct {
			ActionID     string `json:"actionId"`
			PlanRevision int64  `json:"planRevision"`
			PlanDigest   string `json:"planDigest"`
			QuoteDigest  string `json:"quoteDigest"`
		}
		if err = readFullImageJSON(c.Request, &body); err == nil {
			_, err = a.service.ConfirmImagePlan(ctx, imageagent.ConfirmImagePlanInput{RunID: runID, ActionID: body.ActionID, ExpectedRevision: body.PlanRevision, PlanDigest: body.PlanDigest, QuoteDigest: body.QuoteDigest})
		}
	case "preview", "approve":
		var body fullImageSelectionBody
		if err = readFullImageJSON(c.Request, &body); err != nil {
			break
		}
		command, e := fullImageSelection(p, body)
		if e != nil {
			err = e
			break
		}
		preview, e := a.selections.Preview(ctx, command)
		if e != nil {
			err = e
			break
		}
		if action == "preview" {
			c.JSON(http.StatusOK, preview)
			return
		}
		if preview.Digest != body.SelectionDigest {
			err = imageagent.ErrRevisionConflict
			break
		}
		err = a.service.ApproveImageSet(ctx, runID, body.PlanRevision, body.ResultDigest, body.ActionID, command)
	case "restart":
		var body struct {
			PlanRevision int64  `json:"planRevision"`
			PlanDigest   string `json:"planDigest"`
			QuoteDigest  string `json:"quoteDigest"`
		}
		if err = readFullImageJSON(c.Request, &body); err != nil {
			break
		}
		digest, e := imageagent.ImageSetPlanDigest(p.Plan)
		if e != nil || body.PlanRevision != p.Plan.Revision || body.PlanDigest != digest || body.QuoteDigest != p.Plan.Set.QuoteDigest {
			err = imageagent.ErrRevisionConflict
			break
		}
		err = a.service.RestartFailed(ctx, runID)
	case "cancel":
		var body struct {
			ActionID     string `json:"actionId"`
			PlanRevision int64  `json:"planRevision"`
		}
		if err = readFullImageJSON(c.Request, &body); err == nil {
			err = a.service.Cancel(ctx, runID, body.PlanRevision, body.ActionID)
		}
	case "recover":
		var body struct {
			ActionID     string `json:"actionId"`
			PlanRevision int64  `json:"planRevision"`
			SlotID       string `json:"slotId"`
			Attempt      int    `json:"attempt"`
		}
		if err = readFullImageJSON(c.Request, &body); err == nil {
			err = a.service.RecoverEffect(ctx, runID, body.SlotID, body.Attempt, body.PlanRevision, body.ActionID)
		}
	case "resume":
		var body struct {
			ActionID string `json:"actionId"`
		}
		if err = readFullImageJSON(c.Request, &body); err == nil {
			_, err = a.service.Resume(ctx, runID, body.ActionID)
		}
	default:
		err = imageagent.ErrValidation
	}
	if err != nil {
		writeFullImageError(c, err, action != "preview")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"runId": runID, "status": "accepted"})
}

func (a *fullImageApplication) readRecent(ctx context.Context, contextID, cursor string, size int) ([]imageagent.ImageSetRunSummary, string, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || a == nil || a.recent == nil {
		return nil, "", imageagent.ErrIdentityRequired
	}
	exec := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: id.TenantID, UserID: id.UserID, MemberID: id.EffectiveMemberID, BusinessTaskID: contextID}
	items, next, err := a.recent.ListImageSets(ctx, exec, contextID, cursor, size)
	if err != nil {
		return nil, "", err
	}
	result := make([]imageagent.ImageSetRunSummary, 0, len(items))
	for _, item := range items {
		p, e := a.service.Get(ctx, item.RunID)
		if e != nil {
			return nil, "", e
		}
		if e = validateFullImageRun(id, item.ContextKind, item.ContextID, item.RunID, p); e != nil {
			return nil, "", e
		}
		source := p.Plan.Set.Source
		exec.BusinessTaskID = item.ContextID
		_, e = a.readSources.ReadImageSetSource(ctx, exec, imageagent.PrepareImageSetInput{ContextKind: item.ContextKind, ContextID: item.ContextID, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID, Target: imageagent.ImageTargetSelection{Platform: p.Plan.Set.Target.Platform}})
		if e != nil {
			continue
		}
		item.Status = p.Run.Status
		result = append(result, item)
	}
	return result, next, nil
}

func fullImageSelection(p imageagent.RunProjection, body fullImageSelectionBody) (asset.ImageSetCommand, error) {
	if p.Plan.Set == nil || p.Plan.Revision != body.PlanRevision || p.ResultDigest == "" || p.ResultDigest != body.ResultDigest || !acquisitionHTTPUUID(body.ActionID) || len(body.Choices) < 1 || len(body.Choices) > 40 {
		return asset.ImageSetCommand{}, imageagent.ErrCommandBlocked
	}
	source, target := p.Plan.Set.Source, p.Plan.Set.Target
	command := asset.ImageSetCommand{ApprovingResult: asset.ImageSetResultBinding{RunID: p.Run.ID, PlanRevision: p.Plan.Revision, ResultDigest: p.ResultDigest}, ActionID: body.ActionID, Source: asset.SourceSelectionRequest{ContextKind: string(source.ContextKind), ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID, TargetPlatform: target.Platform}, ExpectedHead: body.ExpectedHead, Choices: body.Choices, SelectionDigest: body.SelectionDigest}
	if target.Platform != "product" {
		command.Target = &asset.ImageSetTarget{RecordID: target.RecordID, StoreID: target.StoreID, Site: target.Site, ApplicationID: target.ApplicationID, ApplicationMode: target.ApplicationMode, CategoryID: target.CategoryID, ProductTypeID: target.ProductTypeID, AttributesDigest: target.AttributesDigest, VariantsDigest: target.VariantsDigest}
	}
	return command, nil
}

func (a *fullImageApplication) response(ctx context.Context, p imageagent.RunProjection) (gin.H, error) {
	prepared, err := imageagent.PreparedImageSetFromProjection(p)
	if err != nil {
		return nil, err
	}
	if a.configuration == nil {
		return nil, imageagent.ErrCommandBlocked
	}
	snapshot, err := a.configuration.LoadImageConfiguration(ctx, agent.Scope{OrganizationID: p.Run.TenantID, ActorID: p.Run.UserID}, p.Plan.Set.Configuration)
	if err != nil {
		return nil, err
	}
	template, err := imageSetTemplateReference(p, snapshot)
	if err != nil {
		return nil, err
	}
	slots := make([]gin.H, 0, len(p.Plan.Slots))
	settled := int64(0)
	for _, definition := range p.Plan.Slots {
		slot := imageagent.SlotProjection{Slot: definition}
		for _, current := range p.Slots {
			if current.Slot.ID == definition.ID {
				slot = current
				break
			}
		}
		candidates := make([]gin.H, 0, len(slot.Candidates))
		for index, candidate := range slot.Candidates {
			url, e := acquisitionImagePublishedURL(p, slot.Slot, slot.Attempt, candidate, index, a.publicURLs, a.trial)
			if e != nil {
				return nil, e
			}
			candidates = append(candidates, gin.H{"assetId": candidate.AssetID, "url": url, "width": candidate.Width, "height": candidate.Height})
		}
		if slot.Closure != nil {
			settled += slot.Closure.Points
		}
		slots = append(slots, gin.H{"slotId": definition.ID, "status": slot.Slot.Status, "attempt": slot.Attempt, "errorCode": slot.ErrorCode, "recipe": definition.Recipe, "candidates": candidates, "closure": slot.Closure})
	}
	closed := false
	if p.Run.ImageAdmission != nil && p.PendingCommand == nil && imageagent.ImageSetClosedRunStatus(p.Run.Status) {
		_, e := imageagent.ImageSetClosedEffectsDigest(p.Plan, p.Slots, p.RecoverableEffects)
		closed = e == nil
	}
	response := gin.H{"runId": p.Run.ID, "status": p.Run.Status, "planRevision": p.Plan.Revision, "planDigest": prepared.PlanDigest, "quoteDigest": prepared.QuoteDigest, "images": prepared.Images, "points": prepared.Points, "settledPoints": settled, "plan": p.Plan.Set, "slots": slots, "resultDigest": p.ResultDigest, "originals": p.AssetCatalog.Assets, "recoverableEffects": p.RecoverableEffects, "pendingCommand": p.PendingCommand, "approvalAvailable": p.Run.Status == imageagent.RunStatusAwaitingFinalApproval && p.ResultDigest != "" && closed, "regenerationAvailable": closed, "block": p.Run.Block}
	candidateDigest, candidateErr := imageagent.ImageSetCandidateResultDigest(p)
	response["candidateSelectionAvailable"] = candidateErr == nil
	if candidateErr == nil {
		response["resultDigest"] = candidateDigest
	}
	response["confirmationActionId"], response["generationAdmitted"] = "", p.Run.ImageAdmission != nil
	response["template"] = template
	if p.Run.ImageAdmission != nil {
		response["confirmationActionId"] = p.Run.ImageAdmission.Command.ConfirmActionID
	}
	return response, nil
}

func imageSetTemplateReference(p imageagent.RunProjection, snapshot agentconfig.ImageConfigurationSnapshot) (agentconfig.TemplateRef, error) {
	if p.Plan.Set == nil || snapshot.Ref() != p.Plan.Set.Configuration || snapshot.Scope.OrganizationID != p.Run.TenantID || snapshot.Scope.ActorID != p.Run.UserID || snapshot.MemberID != p.Run.MemberID || snapshot.RunID != p.Run.ID || snapshot.ContextID != p.Run.BusinessTaskID || !agentconfig.UUID(snapshot.Template.TemplateID) {
		return agentconfig.TemplateRef{}, imageagent.ErrCommandBlocked
	}
	return snapshot.Template, nil
}

func readFullImageJSON(request *http.Request, target any) error {
	media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || request.Body == nil || request.Header.Get("Content-Encoding") != "" {
		return imageagent.ErrValidation
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, (128<<10)+1))
	if err != nil || len(raw) == 0 || len(raw) > 128<<10 || !utf8.Valid(raw) {
		return imageagent.ErrValidation
	}
	strict, err := sigjson.UnmarshalStrict(raw, target, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) > 0 {
		return imageagent.ErrValidation
	}
	return nil
}

func writeFullImageError(c *gin.Context, err error, mutation bool) {
	switch {
	case errors.Is(err, review.ErrForbidden), errors.Is(err, collection.ErrForbidden), errors.Is(err, record.ErrForbidden), errors.Is(err, asset.ErrSourceApprovalForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN"})
	case errors.Is(err, collection.ErrInvalid), errors.Is(err, record.ErrInvalid), errors.Is(err, record.ErrTooLarge):
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IMAGE_REQUEST"})
	case errors.Is(err, collection.ErrNotFound), errors.Is(err, record.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "IMAGE_NOT_FOUND"})
	case errors.Is(err, collection.ErrConflict), errors.Is(err, record.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_CONFLICT"})
	case errors.Is(err, record.ErrNotReady):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_BLOCKED"})
	case errors.Is(err, imageagent.ErrBudgetQuoteUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "IMAGE_UNAVAILABLE"})
	case errors.Is(err, asset.ErrInvalidApproval), errors.Is(err, asset.ErrInvalidInventoryScope):
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IMAGE_SELECTION"})
	case errors.Is(err, asset.ErrApprovalConflict):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_SELECTION_CHANGED"})
	case errors.Is(err, asset.ErrApprovedAssetsNotReady):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_ASSETS_NOT_READY"})
	case errors.Is(err, agentconfig.ErrNotEnabled):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_AGENT_DISABLED"})
	case errors.Is(err, agentconfig.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IMAGE_REQUEST"})
	case errors.Is(err, agentconfig.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN"})
	case errors.Is(err, agentconfig.ErrChanged), errors.Is(err, agentconfig.ErrConflict), errors.Is(err, agentconfig.ErrArchived):
		c.JSON(http.StatusConflict, gin.H{"code": "IMAGE_CONFIGURATION_CHANGED"})
	default:
		if mutation {
			writeAcquisitionImageMutationError(c, err)
		} else {
			writeAcquisitionImageError(c, err)
		}
	}
}
