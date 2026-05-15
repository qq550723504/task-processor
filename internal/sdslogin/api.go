package sdslogin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Health(c.Request.Context()))
}

func (h *Handler) Status(c *gin.Context) {
	status, err := h.svc.Status(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	payload, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) ManualLogin(c *gin.Context) {
	var req ManualLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.MerchantName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "merchant_name, username and password are required"})
		return
	}
	payload, err := h.svc.ManualLogin(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) GetAuthState(c *gin.Context) {
	payload, err := h.svc.loadPayload()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) ClearState(c *gin.Context) {
	if err := h.svc.ClearState(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
