package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/accountallocation"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	memberdomain "task-processor/internal/organization/membership"
	memberhttp "task-processor/internal/organization/membership/httpapi"
)

const (
	ModuleName = "account-resource-allocation"
	BasePath   = "/api/v1/account/organization/resources/member-allocations"
	maxBody    = 8 * 1024
)

type Directory interface {
	List(ctx context.Context, organization string, page memberdomain.PageRequest) (memberdomain.Page, error)
	Read(ctx context.Context, organization, id string) (memberdomain.Member, error)
}

type Handler struct {
	service   *accountallocation.Service
	directory Directory
}

func NewHandler(service *accountallocation.Service, directory Directory) (*Handler, error) {
	if service == nil || directory == nil {
		return nil, accountallocation.ErrUnavailable
	}
	return &Handler{service: service, directory: directory}, nil
}

func NewModule(handler *Handler) kernelmodule.Module { return module{handler: handler} }

type module struct{ handler *Handler }

func (module) Name() string { return ModuleName }
func (m module) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m module) Register(reg *kernelmodule.Registry) error {
	if m.handler == nil {
		return accountallocation.ErrUnavailable
	}
	reg.AddRoutes(
		httproute.Descriptor{Method: http.MethodGet, Path: BasePath, Module: ModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: memberhttp.ResolveOrganizationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: m.handler.list},
		httproute.Descriptor{Method: http.MethodPut, Path: BasePath + "/:member_id", Module: ModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberManage, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: memberhttp.ResolveMutationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: m.handler.setTarget},
	)
	return nil
}

func (h *Handler) list(c *gin.Context) {
	identity, ok := identityFromRequest(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	page, err := h.directory.List(c.Request.Context(), identity.EffectiveOrganizationID, memberdomain.PageRequest{Limit: 100})
	if err != nil || page.Total > len(page.Items) {
		writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	activeMemberIDs := make([]string, 0, len(page.Items))
	for _, member := range page.Items {
		if member.State == "active" {
			activeMemberIDs = append(activeMemberIDs, member.ID)
		}
	}
	if err := h.service.RevokeMissingMembers(c.Request.Context(), identity.EffectiveOrganizationID, activeMemberIDs, identity.UserID); err != nil && !errors.Is(err, accountallocation.ErrUnavailable) {
		writeAllocationError(c, err)
		return
	}
	snapshot, err := h.service.Snapshot(c.Request.Context(), identity.EffectiveOrganizationID)
	if err != nil {
		writeAllocationError(c, err)
		return
	}
	byMember := make(map[string]accountallocation.Allocation, len(snapshot.Allocations))
	for _, allocation := range snapshot.Allocations {
		byMember[allocation.MemberID] = allocation
	}
	response := allocationResponse{SchemaVersion: "account-member-token-allocation-v1", OrganizationID: snapshot.OrganizationID, Metric: snapshot.Metric, WindowStart: snapshot.WindowStart.UTC().Format(time.RFC3339Nano), WindowEnd: snapshot.WindowEnd.UTC().Format(time.RFC3339Nano), Enterprise: enterpriseViewResponse(snapshot.Enterprise), Members: make([]memberResponse, 0, len(page.Items))}
	for _, member := range page.Items {
		if member.State != "active" {
			continue
		}
		value := byMember[member.ID]
		if value.MemberID == "" {
			value = accountallocation.Allocation{OrganizationID: snapshot.OrganizationID, MemberID: member.ID, Metric: snapshot.Metric, WindowStart: snapshot.WindowStart, WindowEnd: snapshot.WindowEnd}
		}
		response.Members = append(response.Members, memberResponse{MemberID: member.ID, UserID: member.UserID, DisplayName: member.DisplayName, LoginName: member.LoginName, State: member.State, Allocation: allocationResponseFrom(value)})
	}
	writeJSON(c, http.StatusOK, response)
}

func (h *Handler) setTarget(c *gin.Context) {
	identity, ok := identityFromRequest(c)
	if !ok {
		return
	}
	memberID := strings.TrimSpace(c.Param("member_id"))
	if memberID == "" {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	member, err := h.directory.Read(c.Request.Context(), identity.EffectiveOrganizationID, memberID)
	if err != nil || member.State != "active" {
		writeError(c, http.StatusNotFound, "MEMBER_NOT_FOUND")
		return
	}
	input, err := decodeSetTarget(c.Request)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	target, _ := parseNonnegative(input.Target)
	version, _ := parseNonnegative(input.ExpectedVersion)
	result, err := h.service.SetTarget(c.Request.Context(), accountallocation.SetTargetInput{OrganizationID: identity.EffectiveOrganizationID, MemberID: memberID, Target: target, ExpectedVersion: version, IdempotencyKey: key, ActorID: identity.UserID})
	if err != nil {
		writeAllocationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, allocationResponseFrom(result))
}

type setTargetInput struct {
	Target          string `json:"target"`
	ExpectedVersion string `json:"expectedVersion"`
}

func decodeSetTarget(r *http.Request) (setTargetInput, error) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength <= 0 || r.ContentLength > maxBody || len(r.TransferEncoding) > 0 {
		return setTargetInput{}, errors.New("invalid allocation body")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	decoder.DisallowUnknownFields()
	var value setTargetInput
	if err := decoder.Decode(&value); err != nil {
		return setTargetInput{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return setTargetInput{}, errors.New("trailing allocation body")
	}
	if value.Target == "" || value.ExpectedVersion == "" {
		return setTargetInput{}, errors.New("allocation version and target are required")
	}
	target, err := parseNonnegative(value.Target)
	if err != nil {
		return setTargetInput{}, err
	}
	version, err := parseNonnegative(value.ExpectedVersion)
	if err != nil {
		return setTargetInput{}, err
	}
	value.Target, value.ExpectedVersion = strconv.FormatInt(target, 10), strconv.FormatInt(version, 10)
	return value, nil
}
func parseNonnegative(value string) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, errors.New("invalid nonnegative integer")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, errors.New("invalid nonnegative integer")
	}
	return parsed, nil
}

type allocationResponse struct {
	SchemaVersion  string             `json:"schemaVersion,omitempty"`
	OrganizationID string             `json:"organizationId"`
	Metric         string             `json:"metric"`
	WindowStart    string             `json:"windowStart"`
	WindowEnd      string             `json:"windowEnd"`
	Enterprise     enterpriseResponse `json:"enterprise,omitempty"`
	Members        []memberResponse   `json:"members,omitempty"`
	Allocated      string             `json:"allocated"`
	Consumed       string             `json:"consumed"`
	Remaining      string             `json:"remaining"`
	Version        string             `json:"version"`
	Active         bool               `json:"active"`
}
type enterpriseResponse struct {
	Total       string `json:"total"`
	Allocated   string `json:"allocated"`
	Unallocated string `json:"unallocated"`
	Consumed    string `json:"consumed"`
}
type memberResponse struct {
	MemberID    string             `json:"memberId"`
	UserID      string             `json:"userId"`
	DisplayName string             `json:"displayName"`
	LoginName   string             `json:"loginName"`
	State       string             `json:"state"`
	Allocation  allocationResponse `json:"allocation"`
}

func enterpriseViewResponse(value accountallocation.EnterpriseView) enterpriseResponse {
	return enterpriseResponse{Total: strconv.FormatInt(value.Total, 10), Allocated: strconv.FormatInt(value.Allocated, 10), Unallocated: strconv.FormatInt(value.Unallocated, 10), Consumed: strconv.FormatInt(value.Consumed, 10)}
}
func allocationResponseFrom(value accountallocation.Allocation) allocationResponse {
	result := allocationResponse{OrganizationID: value.OrganizationID, Metric: value.Metric, WindowStart: value.WindowStart.UTC().Format(time.RFC3339Nano), WindowEnd: value.WindowEnd.UTC().Format(time.RFC3339Nano), Allocated: strconv.FormatInt(value.Allocated, 10), Consumed: strconv.FormatInt(value.Consumed, 10), Remaining: strconv.FormatInt(value.Remaining, 10), Version: strconv.FormatInt(value.Version, 10), Active: value.Active}
	return result
}
func identityFromRequest(c *gin.Context) (authidentity.AuthenticatedIdentity, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || identity.EffectiveOrganizationID == "" || identity.UserID == "" {
		writeError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
		return identity, false
	}
	return identity, true
}
func writeJSON(c *gin.Context, status int, value any) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, value)
}
func writeError(c *gin.Context, status int, code string) {
	writeJSON(c, status, gin.H{"code": code, "message": "Account resource allocation request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
func writeAllocationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, accountallocation.ErrConflict), errors.Is(err, accountallocation.ErrIdempotencyConflict):
		writeError(c, http.StatusConflict, "CONFLICT")
	case errors.Is(err, accountallocation.ErrQuotaExceeded):
		writeError(c, http.StatusConflict, "QUOTA_EXCEEDED")
	case errors.Is(err, accountallocation.ErrConsumedFloor):
		writeError(c, http.StatusConflict, "CONSUMED_FLOOR")
	case errors.Is(err, accountallocation.ErrAllocationRequired):
		writeError(c, http.StatusConflict, "ALLOCATION_REQUIRED")
	case errors.Is(err, accountallocation.ErrQuotaUnavailable):
		writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
	default:
		writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
	}
}
