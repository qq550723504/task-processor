package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	b "task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"time"
)

type FinancialPort interface {
	ReadFinancialFacts(context.Context, string) (b.ServiceFinancialView, error)
	RefreshChannelFees(context.Context, string, string) (b.ServiceFinancialView, error)
}

func (h *Handler) SetFinancial(p FinancialPort) { h.financial = p }
func (h *Handler) financialFacts(c *gin.Context, refresh bool) {
	s, ok := scope(c, true)
	if !ok || !noQuery(c) {
		return
	}
	id := c.Param("id")
	if !e.ValidID(id) {
		failure(c, e.ErrInvalid)
		return
	}
	var input struct {
		Date string `json:"date"`
	}
	if refresh {
		if !decode(c, &input) {
			return
		}
		d, err := time.Parse("2006-01-02", input.Date)
		if err != nil || d.Format("2006-01-02") != input.Date {
			failure(c, e.ErrInvalid)
			return
		}
	}
	page, err := h.service.Read(c.Request.Context(), e.Query{Scope: s, Kind: "requests", ID: id, Page: 1, PageSize: 1})
	if err != nil {
		failure(c, err)
		return
	}
	if len(page.Requests) != 1 || page.Requests[0].OrderID == "" {
		failure(c, e.ErrNotFound)
		return
	}
	if h.financial == nil {
		failure(c, e.ErrUnavailable)
		return
	}
	order := page.Requests[0].OrderID
	var v b.ServiceFinancialView
	if refresh {
		v, err = h.financial.RefreshChannelFees(c.Request.Context(), order, input.Date)
	} else {
		v, err = h.financial.ReadFinancialFacts(c.Request.Context(), order)
	}
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, v)
}
