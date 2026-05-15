package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminSensitiveWords(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.ListSensitiveWords(c)
}

func (h *handler) GetAdminSensitiveWord(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.GetSensitiveWord(c)
}

func (h *handler) CreateAdminSensitiveWord(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.CreateSensitiveWord(c)
}

func (h *handler) UpdateAdminSensitiveWord(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.UpdateSensitiveWord(c)
}

func (h *handler) UpdateAdminSensitiveWordStatus(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.UpdateSensitiveWordStatus(c)
}

func (h *handler) DeleteAdminSensitiveWord(c *gin.Context) {
	if !h.requireSensitiveWordHandler(c) {
		return
	}
	h.sensitiveWordHandler.DeleteSensitiveWord(c)
}

func (h *handler) requireSensitiveWordHandler(c *gin.Context) bool {
	if h.sensitiveWordHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "sensitive_word_repository_unavailable",
		"message": "ListingKit sensitive word repository is not configured",
	})
	return false
}
