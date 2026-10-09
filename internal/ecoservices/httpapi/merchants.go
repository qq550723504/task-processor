package httpapi

import (
	"github.com/gin-gonic/gin"
	e "task-processor/internal/ecoservices"
)

func (h *Handler) merchantResume(c *gin.Context) {
	s, ok := scope(c, false)
	if !ok {
		return
	}
	cmd, ok := command(c, s, "merchant_resume")
	if !ok {
		return
	}
	var empty struct{}
	if !decode(c, &empty) {
		return
	}
	if h.merchants == nil {
		failure(c, e.ErrUnavailable)
		return
	}
	v, err := h.merchants.Resume(c.Request.Context(), s, cmd.ID, cmd.Version)
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, v)
}

func (h *Handler) merchantSubmit(c *gin.Context) {
	s, ok := scope(c, false)
	if !ok {
		return
	}
	cmd, ok := command(c, s, "merchant_submit")
	if !ok {
		return
	}
	var details e.MerchantDetails
	if !decode(c, &details) {
		return
	}
	if h.merchants == nil {
		failure(c, e.ErrUnavailable)
		return
	}
	v, err := h.merchants.Submit(c.Request.Context(), s, cmd.Key, cmd.ID, cmd.Version, details)
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, v)
}
func (h *Handler) merchantRead(c *gin.Context) {
	s, ok := scope(c, false)
	if !ok || !noQuery(c) {
		return
	}
	if h.merchants == nil {
		failure(c, e.ErrUnavailable)
		return
	}
	v, err := h.merchants.Read(c.Request.Context(), s, c.Param("id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, v)
}
