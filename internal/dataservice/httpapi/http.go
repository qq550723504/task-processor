package dataservicehttpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/dataservice"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/acquisition/customdata"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"time"
)

const ConsoleBase = "/api/v1/workbench/data-services"
const SpecialistBase = "/api/v1/platform/data-customization"
const APIBase = "/data-api/v1/amazon/jobs"

type createJob struct {
	Query          dataacquisition.Query `json:"query"`
	MaximumRows    int                   `json:"maximumRows"`
	MaximumCostFen int64                 `json:"maximumCostFen"`
}
type keyChange struct {
	ExpectedRevision int64                `json:"expectedRevision"`
	Patch            dataservice.KeyPatch `json:"patch"`
}
type customChange struct {
	ExpectedRevision int64                   `json:"expectedRevision"`
	Patch            dataservice.CustomPatch `json:"patch"`
}

func BuildRoutes(deps Dependencies) []httproute.Descriptor {
	m := &handler{deps}
	specs := []struct {
		method, path, action, permission string
		special, external                bool
	}{
		{"GET", ConsoleBase + "/options", "options", dataservice.PermissionMarket, false, false},
		{"GET", ConsoleBase + "/overview", "overview", dataservice.PermissionManage, false, false},
		{"GET", ConsoleBase + "/keys", "keys", dataservice.PermissionManage, false, false},
		{"POST", ConsoleBase + "/keys", "key-create", dataservice.PermissionManage, false, false},
		{"GET", ConsoleBase + "/keys/history", "key-history", dataservice.PermissionManage, false, false},
		{"GET", ConsoleBase + "/keys/by-command/:command", "key-command", dataservice.PermissionManage, false, false},
		{"POST", ConsoleBase + "/keys/:id/changes", "key-change", dataservice.PermissionManage, false, false},
		{"GET", ConsoleBase + "/amazon/jobs", "jobs", collection.PermissionRead, false, false},
		{"POST", ConsoleBase + "/amazon/jobs", "job-create", dataservice.PermissionMarket, false, false},
		{"GET", ConsoleBase + "/amazon/jobs/by-command/:command", "job-command", collection.PermissionRead, false, false},
		{"GET", ConsoleBase + "/amazon/jobs/:id", "job-read", collection.PermissionRead, false, false},
		{"GET", ConsoleBase + "/amazon/jobs/:id/results", "job-results", collection.PermissionRead, false, false},
		{"POST", ConsoleBase + "/amazon/jobs/:id/cancel", "job-cancel", dataservice.PermissionMarket, false, false},
		{"GET", ConsoleBase + "/custom", "custom-list", dataservice.PermissionMarket, false, false},
		{"POST", ConsoleBase + "/custom", "custom-submit", dataservice.PermissionMarket, false, false},
		{"GET", ConsoleBase + "/custom/by-command/:command", "custom-command", dataservice.PermissionMarket, false, false},
		{"GET", ConsoleBase + "/custom/:id", "custom-read", dataservice.PermissionMarket, false, false},
		{"GET", SpecialistBase, "admin-list", authz.PermissionListingKitPlatformAdm, true, false},
		{"GET", SpecialistBase + "/by-command/:command", "admin-command", authz.PermissionListingKitPlatformAdm, true, false},
		{"GET", SpecialistBase + "/:id", "admin-read", authz.PermissionListingKitPlatformAdm, true, false},
		{"POST", SpecialistBase + "/:id/changes", "admin-change", authz.PermissionListingKitPlatformAdm, true, false},
		{"POST", SpecialistBase + "/:id/delivery", "admin-deliver", authz.PermissionListingKitPlatformAdm, true, false},
		{"POST", APIBase, "job-create", "", false, true},
		{"GET", APIBase + "/by-command/:command", "job-command", "", false, true},
		{"GET", APIBase + "/:id", "job-read", "", false, true},
		{"GET", APIBase + "/:id/results", "job-results", "", false, true},
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, s := range specs {
		auth, org := httproute.AuthPolicyVerifiedIdentity, httproute.OrganizationAccessPolicyLiveWrite
		if s.special {
			auth = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles
			org = httproute.OrganizationAccessPolicyNone
		}
		if s.external {
			auth = httproute.AuthPolicyPublic
			org = httproute.OrganizationAccessPolicyNone
		}
		routes = append(routes, httproute.Descriptor{Method: s.method, Path: s.path, Module: "data-services", Permission: s.permission, AuthPolicy: auth, OrganizationAccessPolicy: org, RequestTimeout: 20 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(3*time.Second, func(c *gin.Context) { m.handle(c, s.action, s.permission, s.special, s.external) })})
	}
	return routes
}
func (m *handler) handle(c *gin.Context, action, permission string, special, external bool) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if m == nil || m.Access == nil {
		dataError(c, dataservice.ErrUnavailable)
		return
	}
	if len(c.Request.URL.RawQuery) > 2048 || c.GetHeader("Content-Encoding") != "" {
		httproute.RejectUnreadRequestBody(c)
		dataError(c, dataservice.ErrInvalid)
		return
	}
	principal := dataacquisition.Principal{}
	var err error
	if external {
		peer, e := externalPeer(c.Request, m.TrustedProxyCIDRs)
		if e != nil {
			dataError(c, e)
			return
		}
		capability := dataservice.PermissionResult
		if action == "job-create" {
			capability = dataservice.PermissionAcquire
		}
		if len(c.Request.Header.Values("Authorization")) != 1 || c.GetHeader("Cookie") != "" || c.GetHeader("X-Requested-Organization-ID") != "" || c.GetHeader("X-Expected-User-ID") != "" {
			dataError(c, dataservice.ErrForbidden)
			return
		}
		p, e := m.Keys.Authenticate(ctx, c.GetHeader("Authorization"), peer, capability)
		if e != nil {
			dataError(c, e)
			return
		}
		principal = dataacquisition.Principal{Scope: p.Scope, CredentialID: p.KeyID, CredentialRevision: p.Revision}
	} else if !special {
		principal.Scope, err = m.Access.Resolve(ctx, permission)
		if err != nil {
			dataError(c, err)
			return
		}
	}
	cursor, limit, state, err := readQuery(c.Request, action)
	if err != nil {
		httproute.RejectUnreadRequestBody(c)
		dataError(c, err)
		return
	}
	command := ""
	if c.Request.Method == "POST" {
		command, err = commandKey(c.Request)
		if err != nil {
			httproute.RejectUnreadRequestBody(c)
			dataError(c, err)
			return
		}
	}
	var value any
	switch action {
	case "options":
		value = m.Options(ctx)
	case "overview":
		err = m.Live.CheckRead(ctx, principal)
		if err != nil {
			break
		}
		var usage dataacquisition.Usage
		usage, err = m.Repository.Usage(ctx, principal.Scope)
		if err == nil {
			var keys []dataservice.Credential
			keys, err = m.Keys.List(ctx)
			if err == nil {
				var jobs []dataacquisition.Job
				jobs, err = m.Repository.List(ctx, principal.Scope, 10)
				if err == nil {
					quotas, e := m.Repository.KeyQuotas(ctx, principal.Scope)
					if e != nil {
						err = e
						break
					}
					value = struct {
						Usage     dataacquisition.Usage      `json:"usage"`
						Keys      []dataservice.Credential   `json:"keys"`
						Jobs      []dataacquisition.Job      `json:"jobs"`
						Options   Options                    `json:"options"`
						KeyQuotas []dataacquisition.KeyQuota `json:"keyQuotas"`
					}{usage, keys, jobs, m.Options(ctx), quotas}
				}
			}
		}
	case "keys":
		value, err = m.Keys.List(ctx)
	case "key-history":
		value, err = m.Keys.History(ctx, cursor, limit)
	case "key-command":
		value, err = m.Keys.Creation(ctx, c.Param("command"))
	case "key-create":
		var input dataservice.KeyInput
		err = readJSON(c.Request, &input)
		if err == nil {
			value, err = m.Keys.Create(ctx, command, input)
		}
	case "key-change":
		var input keyChange
		err = readJSON(c.Request, &input)
		if err == nil {
			value, err = m.Keys.Change(ctx, c.Param("id"), command, input.ExpectedRevision, input.Patch)
		}
	case "jobs":
		err = m.Live.CheckRead(ctx, principal)
		if err == nil {
			value, err = m.Repository.List(ctx, principal.Scope, limit)
		}
	case "job-create":
		var input createJob
		err = readJSON(c.Request, &input)
		if err == nil && input.MaximumRows != input.Query.Limit {
			err = dataacquisition.ErrInvalid
		}
		if err == nil {
			err = m.Provider.Ready(ctx)
		}
		if err == nil {
			var fundingErr error
			funding, fundingErr := m.Funding.Funding(ctx, principal.Scope)
			err = fundingErr
			if err == nil {
				value, err = m.Acquisition.Start(ctx, principal, command, input.Query, funding, input.MaximumCostFen)
			}
		}
	case "job-command":
		if !collection.ValidID(c.Param("command")) {
			err = dataacquisition.ErrInvalid
		} else {
			value, err = m.Acquisition.Read(ctx, principal, collection.StableID(principal.Scope.OrganizationID, principal.Scope.ActorID, "amazon-job", c.Param("command")))
		}
	case "job-read":
		value, err = m.Acquisition.Read(ctx, principal, c.Param("id"))
	case "job-results":
		value, err = m.Acquisition.Results(ctx, principal, c.Param("id"), cursor, limit, m.Results)
	case "job-cancel":
		var input struct{}
		err = readJSON(c.Request, &input)
		if err == nil {
			var job dataacquisition.Job
			job, err = m.Acquisition.Read(ctx, principal, c.Param("id"))
			if err == nil {
				value, err = m.Repository.Cancel(ctx, principal.Scope, job.ID, command)
			}
		}
	case "custom-submit":
		var input dataservice.CustomInput
		err = readJSON(c.Request, &input)
		if err == nil {
			value, err = m.Custom.Submit(ctx, command, input)
		}
	case "custom-command":
		if !collection.ValidID(c.Param("command")) {
			err = dataservice.ErrInvalid
		} else {
			value, err = m.Custom.Read(ctx, collection.StableID(principal.Scope.OrganizationID, principal.Scope.ActorID, "custom-request", c.Param("command")))
		}
	case "custom-list":
		value, err = m.Custom.List(ctx)
	case "custom-read":
		value, err = m.Custom.Read(ctx, c.Param("id"))
	case "admin-list":
		value, err = m.Custom.AdminList(ctx, state, cursor, limit)
	case "admin-read":
		value, err = m.Custom.AdminRead(ctx, c.Param("id"))
	case "admin-command":
		value, err = m.Custom.Command(ctx, c.Param("command"))
	case "admin-change":
		var input customChange
		err = readJSON(c.Request, &input)
		if err == nil {
			value, err = m.Custom.Change(ctx, c.Param("id"), command, input.ExpectedRevision, input.Patch)
		}
	case "admin-deliver":
		if c.Request.Body == nil || c.GetHeader("Content-Type") != "application/octet-stream" {
			err = dataservice.ErrInvalid
			break
		}
		var revision, spec int64
		revision, err = positiveHeader(c.Request, "X-Expected-Revision")
		if err == nil {
			spec, err = positiveHeader(c.Request, "X-Spec-Revision")
		}
		if err == nil {
			format := c.GetHeader("X-Data-Format")
			request, e := m.Custom.AdminRead(ctx, c.Param("id"))
			if e != nil {
				err = e
				break
			}
			if request.Spec == nil || request.Spec.Format != format {
				err = dataservice.ErrConflict
				break
			}
			raw, e := io.ReadAll(io.LimitReader(c.Request.Body, (2<<20)+1))
			err = e
			if err == nil {
				var rows []collection.OwnProduct
				rows, err = customdata.ParseDelivery(ctx, format, raw)
				if err == nil {
					value, err = m.Custom.Deliver(ctx, c.Param("id"), command, revision, spec, rows)
				}
			}
		}
	default:
		err = dataservice.ErrNotFound
	}
	if err != nil {
		dataError(c, err)
		return
	}
	switch typed := value.(type) {
	case []dataservice.CustomRequest:
		items := make([]customSummary, 0, len(typed))
		for _, r := range typed {
			items = append(items, summaryView(r))
		}
		value = items
	case dataservice.CustomPage:
		items := make([]adminSummary, 0, len(typed.Items))
		for _, r := range typed.Items {
			items = append(items, adminSummary{summaryView(r), adminView(r).Applicant})
		}
		value = struct {
			Items      []adminSummary `json:"items"`
			NextCursor string         `json:"nextCursor,omitempty"`
		}{items, typed.NextCursor}
	case dataservice.CustomRequest:
		if special {
			value = adminView(typed)
		}
	}
	raw, e := json.Marshal(value)
	if e != nil || len(raw) > 2<<20 {
		dataError(c, dataservice.ErrUnavailable)
		return
	}
	status := http.StatusOK
	if action == "job-create" {
		status = http.StatusAccepted
	}
	c.Data(status, "application/json", raw)
}

type adminCustom struct {
	dataservice.CustomRequest
	Applicant applicantView `json:"applicant"`
}
type applicantView struct {
	OrganizationID string `json:"organizationId"`
	ActorID        string `json:"actorId"`
}
type customSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Site      string    `json:"site"`
	Mode      string    `json:"mode"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
}
type adminSummary struct {
	customSummary
	Applicant applicantView `json:"applicant"`
}

func summaryView(r dataservice.CustomRequest) customSummary {
	return customSummary{r.ID, r.Input.Name, r.Input.Query.Site, r.Input.Query.Mode, r.State, r.CreatedAt}
}

func adminView(r dataservice.CustomRequest) adminCustom {
	v := adminCustom{CustomRequest: r}
	v.Applicant.OrganizationID = r.Scope.OrganizationID
	v.Applicant.ActorID = r.Scope.ActorID
	return v
}
func commandKey(r *http.Request) (string, error) {
	v := r.Header.Values("Idempotency-Key")
	if len(v) != 1 || !collection.ValidID(v[0]) {
		return "", dataservice.ErrInvalid
	}
	return v[0], nil
}
func readJSON(r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.Body == nil {
		return dataservice.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if err != nil || httproute.DecodeJSON(raw, out, 64<<10, true) != nil {
		return dataservice.ErrInvalid
	}
	return nil
}
func positiveHeader(r *http.Request, name string) (int64, error) {
	v := r.Header.Values(name)
	if len(v) != 1 {
		return 0, dataservice.ErrInvalid
	}
	n, e := strconv.ParseInt(v[0], 10, 64)
	if e != nil || n < 1 {
		return 0, dataservice.ErrInvalid
	}
	return n, nil
}
func readQuery(r *http.Request, action string) (string, int, string, error) {
	if r.Method == "GET" && (r.ContentLength != 0 || len(r.TransferEncoding) > 0) {
		return "", 0, "", dataservice.ErrInvalid
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return "", 0, "", dataservice.ErrInvalid
	}
	allowed := map[string]bool{}
	switch action {
	case "key-history", "job-results":
		allowed["cursor"] = true
		allowed["limit"] = true
	case "admin-list":
		allowed["cursor"] = true
		allowed["limit"] = true
		allowed["state"] = true
	case "jobs":
		allowed["limit"] = true
	}
	for k, v := range q {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			return "", 0, "", dataservice.ErrInvalid
		}
	}
	limit := 100
	if q.Has("limit") {
		limit, e = strconv.Atoi(q.Get("limit"))
		if e != nil || limit < 1 || limit > 100 {
			return "", 0, "", dataservice.ErrInvalid
		}
	}
	cursor := q.Get("cursor")
	if cursor != "" && !collection.ValidID(cursor) {
		return "", 0, "", dataservice.ErrInvalid
	}
	return cursor, limit, q.Get("state"), nil
}
func validateProxyCIDRs(values []string) error {
	if len(values) > 20 {
		return dataservice.ErrInvalid
	}
	for _, v := range values {
		if _, e := netip.ParsePrefix(v); e != nil {
			return dataservice.ErrInvalid
		}
	}
	return nil
}
func externalPeer(r *http.Request, proxies []string) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", dataservice.ErrForbidden
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "", dataservice.ErrForbidden
	}
	trusted := false
	for _, p := range proxies {
		prefix, e := netip.ParsePrefix(p)
		if e == nil && prefix.Contains(peer) {
			trusted = true
		}
	}
	if r.TLS == nil && !(trusted && r.Header.Get("X-Forwarded-Proto") == "https" && len(r.Header.Values("X-Forwarded-Proto")) == 1) {
		return "", dataservice.ErrForbidden
	}
	if trusted {
		v := r.Header.Values("X-Forwarded-For")
		if len(v) != 1 || strings.Contains(v[0], ",") {
			return "", dataservice.ErrForbidden
		}
		ip, e := netip.ParseAddr(v[0])
		if e != nil {
			return "", dataservice.ErrForbidden
		}
		peer = ip
	}
	return peer.String(), nil
}
func dataError(c *gin.Context, err error) {
	status, code := 503, "DATA_UNAVAILABLE"
	switch {
	case errors.Is(err, dataservice.ErrInvalid), errors.Is(err, dataacquisition.ErrInvalid):
		status, code = 400, "INVALID_DATA_REQUEST"
	case errors.Is(err, dataservice.ErrForbidden), errors.Is(err, dataacquisition.ErrForbidden):
		status, code = 403, "FORBIDDEN"
	case errors.Is(err, dataservice.ErrNotFound), errors.Is(err, dataacquisition.ErrNotFound):
		status, code = 404, "DATA_NOT_FOUND"
	case errors.Is(err, dataservice.ErrConflict), errors.Is(err, dataacquisition.ErrConflict):
		status, code = 409, "DATA_CONFLICT"
	case errors.Is(err, dataservice.ErrUnknown), errors.Is(err, dataacquisition.ErrUnknown):
		status, code = 503, "DATA_UNKNOWN"
	}
	httproute.RejectUnreadRequestBody(c)
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": code}})
}
