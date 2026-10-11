package httpapi

import (
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	b "task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/httproute"
	"time"
)

func failure(c *gin.Context, err error) {
	status, code := 503, "ECOSERVICES_UNAVAILABLE"
	switch {
	case errors.Is(err, e.ErrInvalid), errors.Is(err, b.ErrInvalid):
		status, code = 400, "ECOSERVICES_INVALID"
	case errors.Is(err, e.ErrNotFound), errors.Is(err, b.ErrNotFound):
		status, code = 404, "ECOSERVICES_NOT_FOUND"
	case errors.Is(err, e.ErrForbidden), errors.Is(err, b.ErrAuthorizationRevoked):
		status, code = 403, "ECOSERVICES_FORBIDDEN"
	case errors.Is(err, e.ErrConflict), errors.Is(err, e.ErrNotQualified), errors.Is(err, b.ErrConflict), errors.Is(err, b.ErrOrderCancelled), errors.Is(err, b.ErrReconciliationRequired):
		status, code = 409, "ECOSERVICES_CONFLICT"
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "生态服务请求未完成，请刷新状态后重试。", "requestId": c.GetHeader("X-Request-ID")})
}
func scope(c *gin.Context, platform bool) (e.Scope, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !platform && identity.EffectiveOrganizationID == "" {
		failure(c, e.ErrForbidden)
		return e.Scope{}, false
	}
	// Platform=true comes only from the platform descriptor, whose middleware
	// verifies the global permission and clears organization context.
	return e.Scope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID, Platform: platform}, true
}
func positive(raw string) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 1 || strconv.FormatInt(n, 10) != raw {
		return 0, e.ErrInvalid
	}
	return n, nil
}
func noQuery(c *gin.Context) bool {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		failure(c, e.ErrInvalid)
		return false
	}
	return true
}
func decode(c *gin.Context, out any) bool {
	if !noQuery(c) || len(c.Request.Header.Values("Content-Type")) != 1 || c.GetHeader("Content-Encoding") != "" {
		failure(c, e.ErrInvalid)
		return false
	}
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" || len(params) > 1 || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
		failure(c, e.ErrInvalid)
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") || httproute.DecodeJSON(raw, out, 1<<20, true) != nil {
		failure(c, e.ErrInvalid)
		return false
	}
	return true
}
func command(c *gin.Context, s e.Scope, kind string) (e.Command, bool) {
	cmd := e.Command{Scope: s, Kind: kind, ID: c.Param("id")}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !e.ValidID(keys[0]) {
		failure(c, e.ErrInvalid)
		return cmd, false
	}
	cmd.Key = keys[0]
	values := c.Request.Header.Values("If-Match")
	if kind == "application_submit" && len(values) > 0 || kind != "application_submit" && kind != "listing_create" && kind != "request_create" {
		if len(values) != 1 || len(values[0]) < 3 || values[0][0] != '"' || values[0][len(values[0])-1] != '"' {
			failure(c, e.ErrInvalid)
			return cmd, false
		}
		v, err := positive(values[0][1 : len(values[0])-1])
		if err != nil {
			failure(c, err)
			return cmd, false
		}
		cmd.Version = v
	}
	return cmd, true
}
func (h *Handler) read(kind string, platform bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := scope(c, platform)
		if !ok {
			return
		}
		q := e.Query{Scope: s, Kind: kind, ID: c.Param("id"), Page: 1, PageSize: 20}
		if len(c.Request.URL.RawQuery) > 2048 || c.Request.URL.ForceQuery {
			failure(c, e.ErrInvalid)
			return
		}
		values, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			failure(c, e.ErrInvalid)
			return
		}
		for key, vs := range values {
			if len(vs) != 1 {
				failure(c, e.ErrInvalid)
				return
			}
			v := vs[0]
			switch key {
			case "group":
				q.Group = v
			case "stage":
				q.Stage = v
			case "page", "pageSize":
				n, err := positive(v)
				if err != nil || n > 100000 || key == "pageSize" && n > 100 {
					failure(c, e.ErrInvalid)
					return
				}
				if key == "page" {
					q.Page = int(n)
				} else {
					q.PageSize = int(n)
				}
			case "search":
				q.Search = v
			case "state":
				if len(v) > 64 {
					failure(c, e.ErrInvalid)
					return
				}
				q.State = v
			case "category":
				q.Category = e.Category(v)
			case "side":
				if kind != "requests" {
					failure(c, e.ErrInvalid)
					return
				}
				q.Side = v
			case "from", "to":
				if kind != "requests" && kind != "due_orders" {
					failure(c, e.ErrInvalid)
					return
				}
				at, err := time.Parse(time.RFC3339, v)
				if err != nil {
					failure(c, e.ErrInvalid)
					return
				}
				if key == "from" {
					q.From = &at
				} else {
					q.To = &at
				}
			default:
				failure(c, e.ErrInvalid)
				return
			}
		}
		page, err := h.service.Read(c.Request.Context(), q)
		if err != nil {
			failure(c, err)
			return
		}
		c.JSON(200, page)
	}
}

type listingInput struct {
	Category     e.Category `json:"category"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Items        []string   `json:"items"`
	Regions      []string   `json:"regions"`
	Platforms    []string   `json:"platforms"`
	PriceMinor   string     `json:"priceMinor"`
	DeliveryDays int        `json:"deliveryDays"`
}

func (h *Handler) mutate(kind string, platform bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := scope(c, platform)
		if !ok {
			return
		}
		cmd, ok := command(c, s, kind)
		if !ok {
			return
		}
		switch kind {
		case "application_submit":
			var v struct {
				CompanyName        string       `json:"companyName"`
				RegistrationNumber string       `json:"registrationNumber"`
				Categories         []e.Category `json:"categories"`
				Regions            []string     `json:"regions"`
				FileIDs            []string     `json:"fileIds"`
			}
			if !decode(c, &v) {
				return
			}
			cmd.Application = &e.Application{CompanyName: v.CompanyName, RegistrationNumber: v.RegistrationNumber, Categories: v.Categories, Regions: v.Regions, FileIDs: v.FileIDs}
		case "agreement_accept":
			var v struct {
				AgreementVersion string `json:"agreementVersion"`
			}
			if !decode(c, &v) {
				return
			}
			cmd.AgreementVersion = v.AgreementVersion
		case "listing_create", "listing_update":
			var v listingInput
			if !decode(c, &v) {
				return
			}
			amount, err := positive(v.PriceMinor)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.Listing = &e.Listing{Category: v.Category, Title: v.Title, Description: v.Description, Items: v.Items, Regions: v.Regions, Platforms: v.Platforms, PriceMinor: amount, DeliveryDays: v.DeliveryDays}
		case "request_create":
			var v struct {
				Description string   `json:"description"`
				FileIDs     []string `json:"fileIds"`
			}
			if !decode(c, &v) {
				return
			}
			cmd.Description = v.Description
			cmd.FileIDs = v.FileIDs
		case "quote":
			var v struct {
				AmountMinor        string `json:"amountMinor"`
				Scope              string `json:"scope"`
				AcceptanceCriteria string `json:"acceptanceCriteria"`
				DeliveryDays       int    `json:"deliveryDays"`
			}
			if !decode(c, &v) {
				return
			}
			amount, err := positive(v.AmountMinor)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.Quote = &e.Quote{AmountMinor: amount, Scope: v.Scope, AcceptanceCriteria: v.AcceptanceCriteria, DeliveryDays: v.DeliveryDays}
		case "confirm_quote":
			var v struct {
				QuoteVersion   string `json:"quoteVersion"`
				PolicyAccepted string `json:"policyAccepted"`
			}
			if !decode(c, &v) {
				return
			}
			version, err := positive(v.QuoteVersion)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.Quote = &e.Quote{Version: version}
			cmd.PolicyAccepted = v.PolicyAccepted
		case "deliver":
			var v struct {
				Content string   `json:"content"`
				FileIDs []string `json:"fileIds"`
			}
			if !decode(c, &v) {
				return
			}
			cmd.Delivery = &e.Delivery{Content: v.Content, FileIDs: v.FileIDs}
		case "accept", "reject":
			if kind == "accept" {
				var v struct {
					DeliveryVersion string `json:"deliveryVersion"`
				}
				if !decode(c, &v) {
					return
				}
				version, err := positive(v.DeliveryVersion)
				if err != nil {
					failure(c, err)
					return
				}
				cmd.DeliveryVersion = version
			} else {
				var v struct {
					DeliveryVersion string `json:"deliveryVersion"`
					Reason          string `json:"reason"`
				}
				if !decode(c, &v) {
					return
				}
				version, err := positive(v.DeliveryVersion)
				if err != nil {
					failure(c, err)
					return
				}
				cmd.DeliveryVersion = version
				cmd.Reason = v.Reason
			}
		case "refund_propose":
			var v struct {
				AmountMinor string `json:"amountMinor"`
				Reason      string `json:"reason"`
			}
			if !decode(c, &v) {
				return
			}
			amount, err := positive(v.AmountMinor)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.RefundAmountMinor = amount
			cmd.Reason = v.Reason
		case "refund_confirm":
			var v struct {
				RefundVersion string `json:"refundVersion"`
			}
			if !decode(c, &v) {
				return
			}
			version, err := positive(v.RefundVersion)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.RefundVersion = version
		case "refund_review", "refund_review_reject":
			var v struct {
				RefundVersion string `json:"refundVersion"`
				Reason        string `json:"reason"`
			}
			if !decode(c, &v) {
				return
			}
			version, err := positive(v.RefundVersion)
			if err != nil {
				failure(c, err)
				return
			}
			cmd.RefundVersion = version
			cmd.Reason = v.Reason
		case "application_review", "application_reject":
			var v struct {
				Reason string `json:"reason"`
			}
			if !decode(c, &v) {
				return
			}
			cmd.Reason = v.Reason
		case "start", "cancel", "listing_publish":
			if !decode(c, &struct{}{}) {
				return
			}
		default:
			failure(c, e.ErrInvalid)
			return
		}
		result, err := h.service.Mutate(c.Request.Context(), cmd)
		if err != nil {
			failure(c, err)
			return
		}
		c.JSON(200, result)
	}
}
func (h *Handler) checkout(c *gin.Context) {
	if h.payments == nil {
		failure(c, e.ErrUnavailable)
		return
	}
	s, ok := scope(c, false)
	if !ok || !e.ValidID(c.Param("id")) {
		failure(c, e.ErrInvalid)
		return
	}
	if !decode(c, &struct{}{}) {
		return
	}
	qr, err := h.payments.Checkout(c.Request.Context(), s.OrganizationID, s.ActorID, c.Param("id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, gin.H{"orderId": c.Param("id"), "codeUrl": qr})
}
func (h *Handler) upload(parentKind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := scope(c, false)
		if !ok || !noQuery(c) {
			return
		}
		keys := c.Request.Header.Values("Idempotency-Key")
		if len(keys) != 1 || !e.ValidID(keys[0]) || c.GetHeader("Content-Encoding") != "" || len(c.Request.Header.Values("Content-Type")) != 1 {
			failure(c, e.ErrInvalid)
			return
		}
		media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
			failure(c, e.ErrInvalid)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, e.MaxFileBytes+64<<10))
		if err != nil {
			failure(c, e.ErrInvalid)
			return
		}
		reader := multipart.NewReader(strings.NewReader(string(raw)), params["boundary"])
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "file" || part.FileName() == "" {
			failure(c, e.ErrInvalid)
			return
		}
		filename := part.FileName()
		data, err := io.ReadAll(io.LimitReader(part, e.MaxFileBytes+1))
		part.Close()
		if err != nil || len(data) > e.MaxFileBytes {
			failure(c, e.ErrInvalid)
			return
		}
		if _, err := reader.NextPart(); err != io.EOF {
			failure(c, e.ErrInvalid)
			return
		}
		file, err := h.files.Upload(c.Request.Context(), s, keys[0], parentKind, c.Param("id"), filename, data)
		if err != nil {
			failure(c, err)
			return
		}
		c.JSON(200, file)
	}
}
func (h *Handler) download(platform bool, kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := scope(c, platform)
		if !ok || !noQuery(c) {
			return
		}
		file, data, err := h.files.DownloadForKind(c.Request.Context(), s, c.Param("id"), kind)
		if err != nil {
			failure(c, err)
			return
		}
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename}))
		c.Data(200, file.ContentType, data)
	}
}
