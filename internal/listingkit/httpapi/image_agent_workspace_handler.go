package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	listingplatform "task-processor/internal/listing/platform"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

const (
	maxImageAgentWorkspaceRequestBytes = 32 << 10
	// A main-slot quote includes one output each for subject extraction and
	// white-background rendering, even though the run publishes one final image.
	imageAgentWorkspaceMainSlotMaxImages = 2
)

// ImageAgentWorkspaceApplication is the narrow application port used by the
// ListingKit-owned browser entrypoint. It deliberately excludes generic plan
// construction from browser-facing transport code.
type ImageAgentWorkspaceApplication interface {
	Start(context.Context, imageagent.StartRunInput) error
}

// ImageAgentWorkspaceRouteHandler owns the two task-scoped image-agent routes.
// It is separate from the general ListingKit route handler because Image Agent
// is composed later in the process and is optional at runtime.
type ImageAgentWorkspaceRouteHandler interface {
	GetImageAgentAssets(*gin.Context)
	CreateImageAgentRun(*gin.Context)
}

type imageAgentWorkspaceHandler struct {
	tasks       ImageAgentTaskSource
	application ImageAgentWorkspaceApplication
	newID       func() string
}

func NewImageAgentWorkspaceHandler(tasks ImageAgentTaskSource, application ImageAgentWorkspaceApplication) (*imageAgentWorkspaceHandler, error) {
	if tasks == nil {
		return nil, fmt.Errorf("image-agent workspace task source is required")
	}
	if application == nil {
		return nil, fmt.Errorf("image-agent workspace application is required")
	}
	return &imageAgentWorkspaceHandler{tasks: tasks, application: application, newID: uuid.NewString}, nil
}

type imageAgentWorkspaceAssetsResponse struct {
	TargetPlatform  string                     `json:"target_platform,omitempty"`
	SourceAssets    []imageAgentWorkspaceAsset `json:"source_assets"`
	StyleCandidates []imageAgentWorkspaceAsset `json:"style_candidates"`
}

type imageAgentWorkspaceAsset struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DisplayURL string `json:"display_url"`
}

func (h *imageAgentWorkspaceHandler) GetImageAgentAssets(c *gin.Context) {
	task, identity, ok := h.loadOwnedTask(c)
	if !ok {
		return
	}
	_ = identity
	assets, target, err := resolveImageAgentWorkspaceAssets(task, c.Query("target_platform"))
	if err != nil {
		writeImageAgentWorkspaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, imageAgentWorkspaceAssetsResponse{TargetPlatform: target, SourceAssets: assets.sources, StyleCandidates: assets.styles})
}

type createImageAgentWorkspaceRunRequest struct {
	TargetPlatform string   `json:"target_platform"`
	SourceAssetID  string   `json:"source_asset_id"`
	StyleAssetIDs  []string `json:"style_asset_ids"`
}

func (h *imageAgentWorkspaceHandler) CreateImageAgentRun(c *gin.Context) {
	task, _, ok := h.loadOwnedTask(c)
	if !ok {
		return
	}
	var request createImageAgentWorkspaceRunRequest
	if err := decodeStrictImageAgentWorkspaceJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	assets, target, err := resolveImageAgentWorkspaceAssets(task, request.TargetPlatform)
	if err != nil {
		writeImageAgentWorkspaceError(c, err)
		return
	}
	sourceID := strings.TrimSpace(request.SourceAssetID)
	if sourceID == "" || !assets.hasSource(sourceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "source_asset_id must select one task-owned source asset"})
		return
	}
	styleIDs, err := assets.normalizeStyles(request.StyleAssetIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	runID := "image-agent-" + h.newID()
	plan := imageagent.Plan{
		Revision: 1, IdempotencyKey: "image-agent-plan-" + h.newID(),
		SourceAssetIDs: []string{sourceID}, StyleReferenceIDs: styleIDs,
		Slots: []imageagent.Slot{{
			ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{sourceID}, StyleReferenceIDs: styleIDs,
			IdempotencyKey: "image-agent-slot-main-" + h.newID(), Status: imageagent.SlotStatusPending,
		}},
	}
	if err := h.application.Start(c.Request.Context(), imageagent.StartRunInput{
		RunID: runID, BusinessTaskID: task.ID, TargetPlatform: target, Mode: imageagent.RunModeManual,
		IdempotencyKey: "image-agent-run-" + h.newID(), Plan: plan,
		Budget: imageagent.Budget{MaxImages: imageAgentWorkspaceMainSlotMaxImages, EnabledLimits: imageagent.BudgetLimitImages},
	}); err != nil {
		writeImageAgentWorkspaceError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run_id": runID, "status": "accepted"})
}

func (h *imageAgentWorkspaceHandler) loadOwnedTask(c *gin.Context) (*listingkit.Task, authidentity.AuthenticatedIdentity, bool) {
	identity, authenticated := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !authenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "verified identity is required"})
		return nil, authidentity.AuthenticatedIdentity{}, false
	}
	task, err := h.tasks.GetTask(c.Request.Context(), strings.TrimSpace(c.Param("task_id")))
	if err != nil {
		writeImageAgentWorkspaceError(c, err)
		return nil, authidentity.AuthenticatedIdentity{}, false
	}
	if task == nil || task.ID != strings.TrimSpace(c.Param("task_id")) || task.TenantID != identity.TenantID || listingkit.ResolveTaskUserID(task) != identity.UserID {
		c.JSON(http.StatusNotFound, gin.H{"error": "task_not_found", "message": "task is not available to the verified identity"})
		return nil, authidentity.AuthenticatedIdentity{}, false
	}
	return task, identity, true
}

type imageAgentWorkspaceAssets struct {
	sources []imageAgentWorkspaceAsset
	styles  []imageAgentWorkspaceAsset
}

func resolveImageAgentWorkspaceAssets(task *listingkit.Task, targetPlatform string) (imageAgentWorkspaceAssets, string, error) {
	bundle, err := imageAgentBundleForTarget(task, targetPlatform)
	if err != nil {
		return imageAgentWorkspaceAssets{}, "", err
	}
	target := ""
	if task != nil && task.Result != nil && len(task.Result.AssetBundlesByTarget) > 0 {
		target = listingplatform.Normalize(targetPlatform)
	}
	assets := imageAgentWorkspaceAssets{
		sources: make([]imageAgentWorkspaceAsset, 0),
		styles:  make([]imageAgentWorkspaceAsset, 0),
	}
	if bundle == nil {
		return imageAgentWorkspaceAssets{}, "", fmt.Errorf("%w: task has no asset bundle", imageagent.ErrValidation)
	}
	for _, item := range bundle.Assets {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		url, err := imageagent.ValidateSafeImageURL(item.URL)
		if err != nil {
			continue
		}
		label, err := displayLabel(&item)
		if err != nil {
			continue
		}
		candidate := imageAgentWorkspaceAsset{ID: strings.TrimSpace(item.ID), Label: label, DisplayURL: url}
		if item.Kind == asset.KindSourceImage {
			assets.sources = append(assets.sources, candidate)
			continue
		}
		assets.styles = append(assets.styles, candidate)
	}
	if len(assets.sources) == 0 {
		return imageAgentWorkspaceAssets{}, "", fmt.Errorf("%w: task has no safe source images", imageagent.ErrValidation)
	}
	return assets, target, nil
}

func (assets imageAgentWorkspaceAssets) hasSource(id string) bool {
	for _, asset := range assets.sources {
		if asset.ID == id {
			return true
		}
	}
	return false
}

func (assets imageAgentWorkspaceAssets) normalizeStyles(raw []string) ([]string, error) {
	known := make(map[string]struct{}, len(assets.styles))
	for _, asset := range assets.styles {
		known[asset.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, fmt.Errorf("style_asset_ids cannot contain an empty value")
		}
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("style_asset_id %q is not a safe task-owned style candidate", id)
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func decodeStrictImageAgentWorkspaceJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxImageAgentWorkspaceRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON object")
		}
		return err
	}
	return nil
}

func writeImageAgentWorkspaceError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "image_agent_workspace_failed"
	if errors.Is(err, core.ErrTaskNotFound) {
		status, code = http.StatusNotFound, "task_not_found"
	} else if errors.Is(err, imageagent.ErrValidation) {
		status, code = http.StatusBadRequest, "invalid_request"
	} else if errors.Is(err, imageagent.ErrCommandBlocked) {
		status, code = http.StatusConflict, "image_agent_unavailable"
	}
	c.JSON(status, gin.H{"error": code, "message": err.Error()})
}
