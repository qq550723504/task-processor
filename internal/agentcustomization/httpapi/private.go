package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	d "task-processor/internal/agentcustomization"
	"task-processor/internal/httproute"
)

type privatePort interface {
	Deliveries(context.Context, d.Scope, string) (d.DeliveryPage, error)
	Delivery(context.Context, d.Scope, string) (d.Delivery, error)
	RunQuality(context.Context, d.RunCommand) (d.QualityRun, error)
	QualityRuns(context.Context, d.Scope, string, string) (d.QualityRunPage, error)
}

func (h *Handler) servePrivate(c *gin.Context, scope d.Scope, operation string) {
	service, ok := h.Service.(privatePort)
	if !ok {
		failure(c, d.ErrUnavailable)
		return
	}
	query, e := url.ParseQuery(c.Request.URL.RawQuery)
	if e != nil || len(c.Request.URL.RawQuery) > 256 || c.Request.URL.ForceQuery {
		failure(c, d.ErrInvalid)
		return
	}
	cursor := ""
	for k, v := range query {
		if k != "cursor" || len(v) != 1 || !d.UUID(v[0]) || (operation != "deliveries" && operation != "reports") {
			failure(c, d.ErrInvalid)
			return
		}
		cursor = v[0]
	}
	var out any
	switch operation {
	case "deliveries":
		out, e = service.Deliveries(c.Request.Context(), scope, cursor)
	case "delivery":
		out, e = service.Delivery(c.Request.Context(), scope, c.Param("id"))
	case "reports":
		out, e = service.QualityRuns(c.Request.Context(), scope, c.Param("id"), cursor)
	case "run":
		if len(query) > 0 || c.GetHeader("Content-Encoding") != "" || len(c.Request.Header.Values("Content-Type")) != 1 || len(c.Request.Header.Values("If-Match")) > 0 || len(c.Request.Header.Values("If-None-Match")) > 0 {
			failure(c, d.ErrInvalid)
			return
		}
		media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || media != "application/json" || len(params) > 1 || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
			failure(c, d.ErrInvalid)
			return
		}
		keys := c.Request.Header.Values("Idempotency-Key")
		if len(keys) != 1 || !d.UUID(keys[0]) {
			failure(c, d.ErrInvalid)
			return
		}
		command := d.RunCommand{Scope: scope, Key: keys[0], DeliveryID: c.Param("id")}
		raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
		if err != nil || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") || httproute.DecodeJSON(raw, &command.Input, 16<<10, true) != nil {
			failure(c, d.ErrInvalid)
			return
		}
		out, e = service.RunQuality(c.Request.Context(), command)
	default:
		failure(c, d.ErrInvalid)
		return
	}
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, out)
}
