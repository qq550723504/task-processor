package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	a1688 "task-processor/internal/integration/acquisition/a1688"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/sourcing"
)

const acquisitionImageBase = productAcquisitionBase + "/:operation_id/main-image"

type acquisitionImageService interface {
	Start(context.Context, imageagent.StartRunInput) error
	Get(context.Context, string) (imageagent.RunProjection, error)
	ApproveResults(context.Context, string, int64, string, string) error
}

type acquisitionImageCandidateReader interface {
	Candidates(context.Context, imageagent.AssetCatalogScope) ([]imageagent.AuthorizedAsset, error)
}

type acquisitionImageModule struct{ routes []httproute.Descriptor }

func buildAcquisitionImageModule(ctx context.Context, productDB, imageDB *gorm.DB, workflows imageagent.WorkflowClient, dependencies routeAuthDependencies, authorizer *authz.ListingKitAuthorizer, cfg *config.Config) (kernelmodule.Module, error) {
	if productDB == nil || imageDB == nil || workflows == nil || dependencies.organizationResolver == nil || authorizer == nil || cfg == nil ||
		!cfg.ImageAgent.Admission.Enabled || len(cfg.ImageAgent.Admission.AllowedTenantIDs) == 0 {
		return nil, imageagent.ErrIdentityRequired
	}
	publicURLs := imageAgentDurableAssetPublicURLResolver(cfg)
	if publicURLs == nil {
		return nil, imageagent.ErrIdentityRequired
	}
	live := &productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}
	receipts, err := productsourcing.NewPublicAcquisition(ctx, productDB, live, authorizer, a1688.New())
	if err != nil {
		return nil, err
	}
	catalog := organizationImageCatalog{receipts: receipts}
	availability, err := loadImageAgentPolicyAvailability()
	if err != nil {
		return nil, err
	}
	service, err := imageagent.NewService(imagestore.NewOrganizationRepository(imageDB), workflows, catalog,
		imageagent.WithOrganizationScope(),
		imageagent.WithTenantStartGate(imageagent.TenantAllowlistStartGate{Enabled: cfg.ImageAgent.Admission.Enabled, AllowedTenantIDs: append([]string(nil), cfg.ImageAgent.Admission.AllowedTenantIDs...)}),
		imageagent.WithImagePolicyAvailability(availability))
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return acquisitionImageModule{routes: acquisitionImageRoutes(service, catalog, binder.Bind, publicURLs)}, nil
}

func (acquisitionImageModule) Name() string                { return "acquisition-main-image" }
func (acquisitionImageModule) Enabled(*config.Config) bool { return true }
func (m acquisitionImageModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

func acquisitionImageRoutes(service acquisitionImageService, catalog acquisitionImageCandidateReader, bind func(context.Context, string) (context.Context, error), publicURLs imageagent.DurableAssetPublicURLResolver) []httproute.Descriptor {
	specs := []struct{ method, path, permission, action string }{
		{http.MethodGet, acquisitionImageBase + "/candidates", authz.PermissionImageAgentRead, "candidates"},
		{http.MethodPost, acquisitionImageBase, authz.PermissionImageAgentWrite, "start"},
		{http.MethodGet, acquisitionImageBase + "/runs/:run_id", authz.PermissionImageAgentRead, "read"},
		{http.MethodPost, acquisitionImageBase + "/runs/:run_id/approve", authz.PermissionImageAgentWrite, "approve"},
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		spec := spec
		routes = append(routes, httproute.Descriptor{
			Method: spec.method, Path: spec.path, Module: "acquisition-main-image", Permission: spec.permission,
			AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite,
			RequestTimeout: 30 * time.Second, Handler: func(c *gin.Context) {
				if service == nil || catalog == nil || bind == nil || publicURLs == nil {
					writeAcquisitionImageError(c, imageagent.ErrCommandBlocked)
					return
				}
				identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
				if !ok || identity.EffectiveMemberID == "" || identity.TenantID == "" || identity.TenantID != identity.EffectiveOrganizationID {
					writeAcquisitionImageError(c, imageagent.ErrIdentityRequired)
					return
				}
				operationID := c.Param("operation_id")
				if !acquisitionHTTPUUID(operationID) || c.Request.URL.RawQuery != "" {
					writeAcquisitionImageError(c, imageagent.ErrValidation)
					return
				}
				ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
				if err != nil {
					writeAcquisitionImageError(c, imageagent.ErrIdentityRequired)
					return
				}
				switch spec.action {
				case "candidates":
					if !emptyAcquisitionImageBody(c) {
						return
					}
					assets, err := catalog.Candidates(ctx, imageagent.AssetCatalogScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, BusinessTaskID: operationID})
					if err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					candidates := make([]gin.H, 0, len(assets))
					for _, asset := range assets {
						candidates = append(candidates, gin.H{"id": asset.ID, "displayUrl": asset.DisplayURL})
					}
					c.JSON(http.StatusOK, gin.H{"operationId": operationID, "candidates": candidates})
				case "start":
					requestIDs := c.Request.Header.Values("Idempotency-Key")
					if len(requestIDs) != 1 {
						writeAcquisitionImageError(c, imageagent.ErrValidation)
						return
					}
					requestID := requestIDs[0]
					var body struct {
						SourceImageID string `json:"sourceImageId"`
					}
					if err := readAcquisitionImageJSON(c.Request, &body); err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					input, err := acquisitionMainRunInput(identity, operationID, requestID, body.SourceImageID)
					if err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					if err := service.Start(ctx, input); err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					c.JSON(http.StatusAccepted, gin.H{"runId": input.RunID, "status": "accepted"})
				case "read", "approve":
					runID := c.Param("run_id")
					if !acquisitionHTTPUUID(runID) {
						writeAcquisitionImageError(c, imageagent.ErrValidation)
						return
					}
					projection, err := service.Get(ctx, runID)
					if err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					if projection.Run.ScopeProtocol != imageagent.OrganizationScopeProtocol || projection.Run.ID != runID ||
						projection.Run.TenantID != identity.TenantID || projection.Run.UserID != identity.UserID || projection.Run.MemberID != identity.EffectiveMemberID ||
						projection.Run.BusinessTaskID != operationID || projection.Run.TargetPlatform != "product" || projection.Run.ImagePolicyContext != (imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}) {
						writeAcquisitionImageError(c, imageagent.ErrRunNotFound)
						return
					}
					if spec.action == "read" {
						if !emptyAcquisitionImageBody(c) {
							return
						}
						response, err := acquisitionImageResult(projection, publicURLs)
						if err != nil {
							writeAcquisitionImageError(c, err)
							return
						}
						c.JSON(http.StatusOK, response)
						return
					}
					var body struct {
						PlanRevision int64  `json:"planRevision"`
						ResultDigest string `json:"resultDigest"`
						ActionID     string `json:"actionId"`
					}
					if err := readAcquisitionImageJSON(c.Request, &body); err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					if (projection.Run.Status != imageagent.RunStatusAwaitingFinalApproval && projection.Run.Status != imageagent.RunStatusCompleted) || projection.Plan.Revision != body.PlanRevision || projection.ResultDigest == "" || projection.ResultDigest != body.ResultDigest || !acquisitionHTTPUUID(body.ActionID) {
						writeAcquisitionImageError(c, imageagent.ErrCommandBlocked)
						return
					}
					if err := service.ApproveResults(ctx, runID, body.PlanRevision, body.ResultDigest, body.ActionID); err != nil {
						writeAcquisitionImageError(c, err)
						return
					}
					c.JSON(http.StatusAccepted, gin.H{"runId": runID, "status": "accepted"})
				}
			},
		})
	}
	return routes
}

func emptyAcquisitionImageBody(c *gin.Context) bool {
	if c.Request.Body != nil {
		data, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
		if err != nil || len(data) != 0 {
			writeAcquisitionImageError(c, imageagent.ErrValidation)
			return false
		}
	}
	return true
}

func readAcquisitionImageJSON(request *http.Request, target any) error {
	media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) || request.Body == nil || request.Header.Get("Content-Encoding") != "" {
		return imageagent.ErrValidation
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, 8193))
	if err != nil || len(raw) == 0 || len(raw) > 8192 || !utf8.Valid(raw) {
		return imageagent.ErrValidation
	}
	strict, err := sigjson.UnmarshalStrict(raw, target, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) != 0 {
		return imageagent.ErrValidation
	}
	return nil
}

func acquisitionImageResult(projection imageagent.RunProjection, publicURLs imageagent.DurableAssetPublicURLResolver) (gin.H, error) {
	response := gin.H{
		"runId": projection.Run.ID, "status": projection.Run.Status,
		"planRevision": projection.Plan.Revision, "resultDigest": projection.ResultDigest,
		"approvalAvailable": projection.Run.Status == imageagent.RunStatusAwaitingFinalApproval && projection.ResultDigest != "",
	}
	if projection.Run.Block != nil {
		response["blockCode"] = projection.Run.Block.Code
	}
	for _, slot := range projection.Slots {
		if slot.Slot.ID != "main" {
			continue
		}
		for _, candidate := range slot.Candidates {
			url := candidate.URL
			if url == "" {
				asset, err := imageagent.NormalizeDurableAssetIdentity(candidate.DurableAsset)
				if err != nil {
					return nil, err
				}
				url = publicURLs.PublicURL(asset.ObjectKey)
			}
			safe, err := imageagent.ValidateSafeImageURL(url)
			if err != nil {
				return nil, err
			}
			response["imageUrl"] = safe
			return response, nil
		}
	}
	return response, nil
}

func writeAcquisitionImageError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "IMAGE_UNAVAILABLE"
	switch {
	case errors.Is(err, imageagent.ErrIdentityRequired), errors.Is(err, sourcing.ErrPublicationForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, imageagent.ErrValidation), errors.Is(err, sourcing.ErrInvalidAcquisition):
		status, code = http.StatusBadRequest, "INVALID_IMAGE_REQUEST"
	case errors.Is(err, imageagent.ErrRunNotFound), errors.Is(err, sourcing.ErrAcquisitionNotFound):
		status, code = http.StatusNotFound, "IMAGE_NOT_FOUND"
	case errors.Is(err, imageagent.ErrRevisionConflict), errors.Is(err, sourcing.ErrAcquisitionConflict):
		status, code = http.StatusConflict, "IMAGE_CONFLICT"
	case errors.Is(err, imageagent.ErrCommandBlocked):
		status, code = http.StatusConflict, "IMAGE_BLOCKED"
	}
	c.JSON(status, gin.H{"code": code})
}

// acquisitionMainRunInput is the server-owned single-result plan for the
// current 1688 product detail page. Browser input never supplies a policy,
// catalog URL, workflow plan, budget, or member allocation identity.
func acquisitionMainRunInput(identity authidentity.AuthenticatedIdentity, operationID, requestID, sourceID string) (imageagent.StartRunInput, error) {
	if identity.TenantID == "" || identity.EffectiveOrganizationID != identity.TenantID || identity.UserID == "" || identity.EffectiveMemberID == "" ||
		!acquisitionHTTPUUID(operationID) || !acquisitionHTTPUUID(requestID) || !validAcquisitionSourceID(sourceID) {
		return imageagent.StartRunInput{}, imageagent.ErrValidation
	}
	name := strings.Join([]string{identity.TenantID, identity.UserID, identity.EffectiveMemberID, operationID, requestID}, "\x00")
	runID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)).String()
	plan := imageagent.Plan{
		Revision: 1, IdempotencyKey: runID + "-plan", SourceAssetIDs: []string{sourceID},
		Slots: []imageagent.Slot{{
			ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{sourceID},
			IdempotencyKey: runID + "-slot-main", Status: imageagent.SlotStatusPending,
		}},
	}
	return imageagent.StartRunInput{
		RunID: runID, BusinessTaskID: operationID, TargetPlatform: "product",
		ImagePolicyContext: imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"},
		Mode:               imageagent.RunModeManual, IdempotencyKey: runID + "-request", Plan: plan,
		// The v3 main quote counts Extract, RenderWhiteBackground and Review
		// separately. The plan still permits only one finished main image.
		Budget: imageagent.Budget{MaxImages: 3, EnabledLimits: imageagent.BudgetLimitImages}, MaxConcurrentSlots: 1,
	}, nil
}

func validAcquisitionSourceID(value string) bool {
	if !strings.HasPrefix(value, "catalog-image-") || len(value) > 64 {
		return false
	}
	digits := strings.TrimPrefix(value, "catalog-image-")
	if digits == "" || digits[0] < '1' || digits[0] > '9' {
		return false
	}
	for _, digit := range digits[1:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
