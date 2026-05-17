package listingadmin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type SensitiveWordHandler struct{ repo SensitiveWordRepository }

func NewSensitiveWordHandler(repo SensitiveWordRepository) *SensitiveWordHandler {
	return &SensitiveWordHandler{repo: repo}
}

func (h *SensitiveWordHandler) ListSensitiveWords(c *gin.Context) {
	query := SensitiveWordQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestUserID(c),
		Page:        queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:    queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Word:        strings.TrimSpace(c.Query("word")),
		Language:    strings.TrimSpace(c.Query("language")),
		Tags:        strings.TrimSpace(c.Query("tags")),
		Remark:      strings.TrimSpace(c.Query("remark")),
	}
	query.Level = queryIntPtr(c, "level")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListSensitiveWords(withRequestUserID(c.Request.Context(), requestUserID(c)), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sensitive_word_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *SensitiveWordHandler) GetSensitiveWord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	word, err := h.repo.GetSensitiveWord(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id)
	if err != nil {
		writeSensitiveWordError(c, err, "sensitive_word_get_failed")
		return
	}
	c.JSON(http.StatusOK, word)
}

func (h *SensitiveWordHandler) CreateSensitiveWord(c *gin.Context) {
	var req SensitiveWord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateSensitiveWord(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_sensitive_word", "message": err.Error()})
		return
	}
	word, err := h.repo.CreateSensitiveWord(withRequestUserID(c.Request.Context(), requestUserID(c)), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sensitive_word_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, word)
}

func (h *SensitiveWordHandler) UpdateSensitiveWord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req SensitiveWord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateSensitiveWord(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_sensitive_word", "message": err.Error()})
		return
	}
	word, err := h.repo.UpdateSensitiveWord(withRequestUserID(c.Request.Context(), requestUserID(c)), &req)
	if err != nil {
		writeSensitiveWordError(c, err, "sensitive_word_update_failed")
		return
	}
	c.JSON(http.StatusOK, word)
}

func (h *SensitiveWordHandler) UpdateSensitiveWordStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16  `json:"status"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	word, err := h.repo.UpdateSensitiveWordStatus(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeSensitiveWordError(c, err, "sensitive_word_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, word)
}

func (h *SensitiveWordHandler) DeleteSensitiveWord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteSensitiveWord(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id); err != nil {
		writeSensitiveWordError(c, err, "sensitive_word_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateSensitiveWord(word *SensitiveWord) error {
	switch {
	case word.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(word.Word) == "":
		return errors.New("word is required")
	case strings.TrimSpace(word.Language) == "":
		return errors.New("language is required")
	case word.Level < 0:
		return errors.New("level cannot be negative")
	}
	return nil
}

func queryIntPtr(c *gin.Context, key string) *int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func writeSensitiveWordError(c *gin.Context, err error, code string) {
	if errors.Is(err, ErrSensitiveWordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sensitive_word_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
