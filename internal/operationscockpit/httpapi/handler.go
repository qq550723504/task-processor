package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"net/url"
	strict "sigs.k8s.io/json"
	"sort"
	"strconv"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	c "task-processor/internal/operationscockpit"
	"time"
)

const Base = "/api/v1/workbench/operations-cockpit"
const ModuleName = "operations-cockpit"

type Handler struct {
	Repository   c.Repository
	Bind         func(context.Context, string) (context.Context, error)
	Scope        func(context.Context) (c.Scope, error)
	Current      func(context.Context, c.Scope) (c.Access, error)
	Directory    func(context.Context, c.Scope) ([]c.StoreReference, error)
	Revalidate   func(context.Context, c.Scope, []string) error
	Now          func() time.Time
	Observations func(context.Context, c.Scope, []c.StoreReference) (c.ObservationProjection, error)
}

func BuildRoutes(h *Handler) ([]httproute.Descriptor, error) {
	if h == nil || h.Repository == nil || h.Bind == nil || h.Scope == nil || h.Current == nil || h.Directory == nil || h.Revalidate == nil {
		return nil, c.ErrUnavailable
	}
	return Routes(h), nil
}
func Routes(h *Handler) []httproute.Descriptor {
	result := []httproute.Descriptor{}
	for _, s := range []struct{ method, path, permission string }{
		{"GET", "/capabilities", authz.PermissionWorkbenchStoreRead},
		{"GET", "/stores", authz.PermissionCockpitStoresRead},
		{"GET", "/stores/:id", authz.PermissionCockpitStoresRead},
		{"GET", "/facts", authz.PermissionCockpitStoresRead},
		{"GET", "/facts/:id", authz.PermissionCockpitStoresRead},
		{"GET", "/facts/:id/history", authz.PermissionCockpitStoresRead},
		{"POST", "/facts", authz.PermissionCockpitFactsWrite},
		{"GET", "/goals", authz.PermissionCockpitGoalsRead},
		{"GET", "/goals/head", authz.PermissionCockpitGoalsRead},
		{"GET", "/goals/history", authz.PermissionCockpitGoalsRead},
		{"POST", "/goals", authz.PermissionCockpitGoalsRead},
		{"POST", "/goals/restore", authz.PermissionCockpitGoalsRead},
		{"GET", "/alerts", authz.PermissionCockpitAlertsRead},
		{"GET", "/advice", authz.PermissionCockpitAdviceRead},
	} {
		path := s.path
		result = append(result, httproute.Descriptor{Method: s.method, Path: Base + s.path, Module: ModuleName, Permission: s.permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, func(ctx *gin.Context) {
			if h == nil || h.Repository == nil || h.Bind == nil || h.Scope == nil || h.Current == nil || h.Directory == nil {
				failure(ctx, c.ErrUnavailable)
				return
			}
			h.serve(ctx, path)
		})})
	}
	return result
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, r := range Routes(nil) {
		if d.Method == r.Method && d.Path == r.Path && d.Module == r.Module && d.Permission == r.Permission && d.AuthPolicy == r.AuthPolicy && d.OrganizationAccessPolicy == r.OrganizationAccessPolicy && d.RequestTimeout == r.RequestTimeout && d.RejectUnreadRequestBody == r.RejectUnreadRequestBody && d.OrganizationTargetResolver == nil && d.Handler != nil {
			return nil
		}
	}
	return errors.New("cockpit route loses current authorization or bounded input")
}
func failure(ctx *gin.Context, err error) {
	code, status := c.ErrUnavailable.Error(), http.StatusServiceUnavailable
	for _, e := range []error{c.ErrInvalid, c.ErrForbidden, c.ErrNotFound, c.ErrRevision, c.ErrConflict, c.ErrOverlap, c.ErrAmountRange} {
		if errors.Is(err, e) {
			code = e.Error()
			status = map[error]int{c.ErrInvalid: 400, c.ErrForbidden: 403, c.ErrNotFound: 404, c.ErrRevision: 412, c.ErrConflict: 409, c.ErrOverlap: 409, c.ErrAmountRange: 422}[e]
			break
		}
	}
	ctx.Header("Cache-Control", "private, no-store")
	httproute.RejectUnreadRequestBody(ctx)
	ctx.AbortWithStatusJSON(status, gin.H{"code": code})
}
func reply(ctx *gin.Context, result any) {
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > 1<<20 {
		failure(ctx, c.ErrUnavailable)
		return
	}
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Data(200, "application/json; charset=utf-8", raw)
}
func body(ctx *gin.Context, result any) error {
	media, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || media != "application/json" || ctx.GetHeader("Content-Encoding") != "" {
		return c.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, (16<<10)+1))
	if err != nil || len(raw) > 16<<10 || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return c.ErrInvalid
	}
	violations, err := strict.UnmarshalStrict(raw, result)
	if err != nil || len(violations) > 0 {
		return c.ErrInvalid
	}
	return nil
}
func query(ctx *gin.Context, allowed ...string) error {
	values, err := urlQuery(ctx)
	if err != nil {
		return err
	}
	for key, items := range values {
		found := false
		for _, v := range allowed {
			if v == key {
				found = true
				break
			}
		}
		if !found || len(items) != 1 && key != "storeId" {
			return c.ErrInvalid
		}
	}
	return nil
}
func urlQuery(ctx *gin.Context) (map[string][]string, error) {
	if len(ctx.Request.URL.RawQuery) > 16<<10 {
		return nil, c.ErrInvalid
	}
	values, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		return nil, c.ErrInvalid
	}
	return values, nil
}
func revision(value string, zero bool) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 || !zero && n == 0 || strconv.FormatInt(n, 10) != value {
		return 0, c.ErrInvalid
	}
	return n, nil
}
func page(ctx *gin.Context) (int, error) {
	value := ctx.Query("page")
	if value == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 1000 || strconv.Itoa(n) != value {
		return 0, c.ErrInvalid
	}
	return n, nil
}
func before(ctx *gin.Context) (int64, error) {
	value := ctx.Query("beforeRevision")
	if value == "" {
		return 0, nil
	}
	return revision(value, false)
}

type amountBody struct {
	Revenue     *int64 `json:"revenue"`
	Refunds     *int64 `json:"refunds"`
	Procurement *int64 `json:"procurement"`
	Logistics   *int64 `json:"logistics"`
	Platform    *int64 `json:"platform"`
	Advertising *int64 `json:"advertising"`
	Other       *int64 `json:"other"`
}

func (a amountBody) amount() (c.Amounts, error) {
	if a.Revenue == nil || a.Refunds == nil || a.Procurement == nil || a.Logistics == nil || a.Platform == nil || a.Advertising == nil || a.Other == nil {
		return c.Amounts{}, c.ErrInvalid
	}
	return c.Amounts{Revenue: *a.Revenue, Refunds: *a.Refunds, Procurement: *a.Procurement, Logistics: *a.Logistics, Platform: *a.Platform, Advertising: *a.Advertising, Other: *a.Other}, nil
}

type factBody struct {
	ID       string `json:"id"`
	StoreID  string `json:"storeId"`
	Expected string `json:"expectedRevision"`
	Fact     *struct {
		Period  c.Period   `json:"period"`
		Amounts amountBody `json:"amounts"`
		Note    string     `json:"note"`
	} `json:"fact"`
}
type goalBody struct {
	ID       string        `json:"id"`
	Expected string        `json:"expectedRevision"`
	Goal     *c.GoalConfig `json:"goal"`
	Source   string        `json:"sourceRevision"`
}

func (h *Handler) write(ctx *gin.Context, path string, scope c.Scope) {
	if err := query(ctx); err != nil {
		failure(ctx, err)
		return
	}
	keys := ctx.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !c.UUID(keys[0]) {
		failure(ctx, c.ErrInvalid)
		return
	}
	cmd := c.Command{Scope: scope, Key: keys[0]}
	var err error
	if path == "/facts" {
		var input factBody
		err = body(ctx, &input)
		if err == nil && input.Fact == nil {
			err = c.ErrInvalid
		}
		if err == nil {
			cmd.ID = input.ID
			cmd.StoreID = input.StoreID
			cmd.Expected, err = revision(input.Expected, true)
			amounts, e := input.Fact.Amounts.amount()
			if e != nil {
				err = e
			}
			cmd.Fact = &c.FactInput{Period: input.Fact.Period, Amounts: amounts, Note: input.Fact.Note}
			cmd.Operation = "fact_update"
			if cmd.Expected == 0 {
				cmd.Operation = "fact_create"
			}
		}
	} else {
		var input goalBody
		err = body(ctx, &input)
		if err == nil {
			cmd.ID = input.ID
			cmd.Expected, err = revision(input.Expected, path == "/goals")
			cmd.Goal = input.Goal
			cmd.Operation = "goal_update"
			if cmd.Expected == 0 {
				cmd.Operation = "goal_create"
			}
			if path == "/goals/restore" {
				cmd.Operation = "goal_restore"
				cmd.SourceRevision, err = revision(input.Source, false)
			} else if input.Source != "" {
				err = c.ErrInvalid
			}
		}
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if err == nil && !cmd.Valid(now) {
		err = c.ErrInvalid
	}
	if err != nil {
		failure(ctx, err)
		return
	}
	result, err := h.Repository.Execute(ctx.Request.Context(), cmd)
	if err != nil {
		failure(ctx, err)
		return
	}
	reply(ctx, result)
}

func (h *Handler) serve(g *gin.Context, path string) {
	requestContext, cancel := context.WithTimeout(g.Request.Context(), 10*time.Second)
	defer cancel()
	bound, err := h.Bind(requestContext, g.GetHeader("Authorization"))
	if err != nil {
		failure(g, err)
		return
	}
	g.Request = g.Request.WithContext(bound)
	scope, err := h.Scope(bound)
	if err != nil {
		failure(g, err)
		return
	}
	if g.Request.Method == "POST" {
		h.write(g, path, scope)
		return
	}
	access, err := h.Current(bound, scope)
	if err != nil {
		failure(g, err)
		return
	}
	if path == "/capabilities" {
		if err := query(g); err != nil {
			failure(g, err)
			return
		}
		if !access.GoalsRead && !access.StoresRead && !access.AlertsRead && !access.AdviceRead {
			failure(g, c.ErrForbidden)
			return
		}
		stores, err := h.Directory(bound, scope)
		if err != nil {
			failure(g, err)
			return
		}
		fresh, err := h.Current(bound, scope)
		if err != nil || fresh != access {
			failure(g, c.ErrForbidden)
			return
		}
		now := time.Now()
		if h.Now != nil {
			now = h.Now()
		}
		reply(g, gin.H{"access": access, "today": c.Today(now), "stores": stores})
		return
	}
	if path == "/goals/head" {
		if err := query(g); err != nil {
			failure(g, err)
			return
		}
		v, err := h.Repository.HeadMetadata(bound, scope)
		if err != nil {
			failure(g, err)
			return
		}
		reply(g, v)
		return
	}
	if path == "/goals/history" || path == "/facts/:id/history" {
		if err := query(g, "beforeRevision"); err != nil {
			failure(g, err)
			return
		}
		n, err := before(g)
		if err != nil {
			failure(g, err)
			return
		}
		if path == "/goals/history" {
			v, err := h.Repository.GoalHistory(bound, scope, n)
			if err != nil {
				failure(g, err)
				return
			}
			reply(g, v)
		} else {
			v, err := h.Repository.FactHistory(bound, scope, g.Param("id"), n)
			if err != nil {
				failure(g, err)
				return
			}
			reply(g, v)
		}
		return
	}
	if path == "/facts/:id" {
		if err := query(g); err != nil {
			failure(g, err)
			return
		}
		v, err := h.Repository.Fact(bound, scope, g.Param("id"))
		if err != nil {
			failure(g, err)
			return
		}
		reply(g, v)
		return
	}
	if path == "/facts" {
		if err := query(g, "storeId", "page"); err != nil {
			failure(g, err)
			return
		}
		if len(g.QueryArray("storeId")) != 1 {
			failure(g, c.ErrInvalid)
			return
		}
		n, err := page(g)
		if err != nil {
			failure(g, err)
			return
		}
		v, err := h.Repository.Facts(bound, scope, g.Query("storeId"), n)
		if err != nil {
			failure(g, err)
			return
		}
		reply(g, v)
		return
	}
	h.projection(g, path, scope, access)
}

type matrixRow struct {
	Store         c.StoreReference `json:"store"`
	Complete      bool             `json:"complete"`
	State         string           `json:"state"`
	Totals        c.Totals         `json:"totals"`
	Growth        *c.Rational      `json:"growth"`
	GapCount      int              `json:"gapCount"`
	ExcludedCount int              `json:"excludedCount"`
	RecordCount   int              `json:"recordCount"`
}

type goalStoreEvidence struct {
	StoreID       string             `json:"storeId"`
	Period        c.Period           `json:"period"`
	Complete      bool               `json:"complete"`
	Totals        c.Totals           `json:"totals"`
	Records       []c.RecordEvidence `json:"records"`
	Excluded      []c.RecordEvidence `json:"excluded"`
	Gaps          []c.Period         `json:"gaps"`
	RecordCount   int                `json:"recordCount"`
	ExcludedCount int                `json:"excludedCount"`
	GapCount      int                `json:"gapCount"`
}

func goalEvidence(snapshot c.Snapshot) []goalStoreEvidence {
	result := []goalStoreEvidence{}
	if snapshot.Goal == nil {
		return result
	}
	for _, id := range snapshot.Goal.Config.StoreIDs {
		a, ok := snapshot.GoalStores[id]
		if !ok {
			continue
		}
		result = append(result, goalStoreEvidence{StoreID: id, Period: a.Period, Complete: a.Complete, Totals: a.Totals, Records: append([]c.RecordEvidence{}, a.Records[:min(len(a.Records), 20)]...), Excluded: append([]c.RecordEvidence{}, a.Excluded[:min(len(a.Excluded), 20)]...), Gaps: append([]c.Period{}, a.Gaps[:min(len(a.Gaps), 20)]...), RecordCount: len(a.Records), ExcludedCount: len(a.Excluded), GapCount: len(a.Gaps)})
	}
	return result
}

func (h *Handler) projection(g *gin.Context, path string, scope c.Scope, access c.Access) {
	if err := query(g, "startDate", "endDate", "storeId", "page", "state", "search", "sort", "level", "kind"); err != nil {
		failure(g, err)
		return
	}
	module := strings.TrimPrefix(path, "/")
	if path == "/stores/:id" {
		module = "stores"
	}
	if !access.Allows(module) {
		failure(g, c.ErrForbidden)
		return
	}
	p := c.Period{Start: g.Query("startDate"), End: g.Query("endDate")}
	if !p.Valid() {
		failure(g, c.ErrInvalid)
		return
	}
	n, err := page(g)
	if err != nil {
		failure(g, err)
		return
	}
	stores, err := h.Directory(g.Request.Context(), scope)
	if err != nil {
		failure(g, err)
		return
	}
	ids := g.QueryArray("storeId")
	if path == "/stores/:id" {
		if len(ids) != 0 {
			failure(g, c.ErrInvalid)
			return
		}
		ids = []string{g.Param("id")}
	}
	names := map[string]c.StoreReference{}
	for _, s := range stores {
		names[s.ID] = s
	}
	if len(ids) == 0 {
		for _, s := range stores {
			ids = append(ids, s.ID)
		}
	} else {
		for _, id := range ids {
			if _, ok := names[id]; !ok {
				failure(g, c.ErrForbidden)
				return
			}
		}
	}
	snapshot, err := h.Repository.Snapshot(g.Request.Context(), scope, c.Query{Module: module, Period: p, StoreIDs: ids})
	if err != nil {
		failure(g, err)
		return
	}
	if path == "/stores/:id" {
		reply(g, gin.H{"store": names[ids[0]], "aggregate": snapshot.Stores[ids[0]], "growth": c.Growth(snapshot.Stores[ids[0]], snapshot.Previous[ids[0]]), "capturedAt": snapshot.CapturedAt})
		return
	}
	if path == "/goals" {
		reply(g, gin.H{"goal": snapshot.Goal, "evaluation": snapshot.Evaluation, "head": snapshot.Head, "goalUnavailable": snapshot.GoalUnavailable, "capturedAt": snapshot.CapturedAt, "basis": goalEvidence(snapshot)})
		return
	}
	if module == "alerts" || module == "advice" {
		observation := c.ObservationProjection{State: "unavailable", Sources: []c.ObservationEvidence{}, Latest: []c.ObservationEvidence{}, ActionPath: "/workbench/store-orders"}
		if h.Observations != nil {
			selected := []c.StoreReference{}
			for _, id := range ids {
				selected = append(selected, names[id])
			}
			observation, err = h.Observations(g.Request.Context(), scope, selected)
			if err != nil {
				failure(g, err)
				return
			}
		}
		fresh, err := h.Current(g.Request.Context(), scope)
		if err != nil || !fresh.Allows(module) || snapshot.Goal != nil && !fresh.GoalsRead {
			failure(g, c.ErrForbidden)
			return
		}
		finalIDs := append([]string(nil), ids...)
		if snapshot.Goal != nil {
			for _, id := range snapshot.Goal.Config.StoreIDs {
				found := false
				for _, selected := range finalIDs {
					if selected == id {
						found = true
						break
					}
				}
				if !found {
					finalIDs = append(finalIDs, id)
				}
			}
		}
		if err := h.Revalidate(g.Request.Context(), scope, finalIDs); err != nil {
			failure(g, err)
			return
		}
		rules, err := c.Rules(snapshot)
		if err != nil {
			failure(g, err)
			return
		}
		filtered := []c.Rule{}
		for _, r := range rules {
			if g.Query("kind") != "" && r.Kind != g.Query("kind") || g.Query("level") != "" && r.Level != g.Query("level") {
				continue
			}
			filtered = append(filtered, r)
		}
		start := min((n-1)*50, len(filtered))
		end := min(start+50, len(filtered))
		reply(g, gin.H{"rules": filtered[start:end], "total": len(filtered), "page": n, "capturedAt": snapshot.CapturedAt, "goalUnavailable": snapshot.GoalUnavailable, "observations": observation})
		return
	}
	rows := []matrixRow{}
	states := map[string]int{"loss": 0, "break_even": 0, "recorded_complete": 0, "data_incomplete": 0}
	summary := c.Totals{}
	allComplete := true
	for _, id := range ids {
		a := snapshot.Stores[id]
		state := "data_incomplete"
		if a.Complete {
			state = "recorded_complete"
			if a.Totals.NetProfit < 0 {
				state = "loss"
			} else if a.Totals.NetProfit == 0 {
				state = "break_even"
			}
		} else {
			allComplete = false
		}
		states[state]++
		if a.Complete {
			summary, err = summary.Add(a.Totals)
			if err != nil {
				failure(g, err)
				return
			}
		}
		if g.Query("state") != "" && state != g.Query("state") || g.Query("search") != "" && !strings.Contains(strings.ToLower(names[id].Name), strings.ToLower(g.Query("search"))) {
			continue
		}
		rows = append(rows, matrixRow{Store: names[id], Complete: a.Complete, State: state, Totals: a.Totals, Growth: c.Growth(a, snapshot.Previous[id]), GapCount: len(a.Gaps), ExcludedCount: len(a.Excluded), RecordCount: len(a.Records)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Complete != rows[j].Complete {
			return rows[i].Complete
		}
		if g.Query("sort") == "profit" && rows[i].Complete && rows[i].Totals.NetProfit != rows[j].Totals.NetProfit {
			return rows[i].Totals.NetProfit > rows[j].Totals.NetProfit
		}
		return rows[i].Store.Name+rows[i].Store.ID < rows[j].Store.Name+rows[j].Store.ID
	})
	start := min((n-1)*50, len(rows))
	end := min(start+50, len(rows))
	reply(g, gin.H{"rows": rows[start:end], "total": len(rows), "page": n, "summary": summary, "summaryComplete": allComplete && len(ids) > 0, "states": states, "capturedAt": snapshot.CapturedAt})
}
