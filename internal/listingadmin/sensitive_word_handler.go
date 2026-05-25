package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type SensitiveWordHandler struct{ repo SensitiveWordRepository }

func NewSensitiveWordHandler(repo SensitiveWordRepository) *SensitiveWordHandler {
	return &SensitiveWordHandler{repo: repo}
}

func (h *SensitiveWordHandler) ListSensitiveWords(c *gin.Context) {
	scope := requestListScope(c)
	query := applyListQueryScope(&SensitiveWordQuery{
		Word:     strings.TrimSpace(c.Query("word")),
		Language: strings.TrimSpace(c.Query("language")),
		Tags:     strings.TrimSpace(c.Query("tags")),
		Remark:   strings.TrimSpace(c.Query("remark")),
	}, scope)
	query.Level = queryIntPtr(c, "level")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListSensitiveWords(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "sensitive_word_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *SensitiveWordHandler) GetSensitiveWord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	word, err := h.repo.GetSensitiveWord(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeSensitiveWordError(c, err, "sensitive_word_get_failed")
		return
	}
	c.JSON(http.StatusOK, word)
}

func (h *SensitiveWordHandler) CreateSensitiveWord(c *gin.Context) {
	var req SensitiveWord
	if !bindAndValidateJSON(c, &req, "invalid_sensitive_word", func(value *SensitiveWord) {
		value.TenantID = requestTenantID(c)
	}, validateSensitiveWord) {
		return
	}
	word, err := h.repo.CreateSensitiveWord(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "sensitive_word_create_failed", err)
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
	if !bindAndValidateJSON(c, &req, "invalid_sensitive_word", func(value *SensitiveWord) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateSensitiveWord) {
		return
	}
	word, err := h.repo.UpdateSensitiveWord(requestIdentityContext(c), &req)
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
	if !bindJSON(c, &req) {
		return
	}
	word, err := h.repo.UpdateSensitiveWordStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
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
	if err := h.repo.DeleteSensitiveWord(requestIdentityContext(c), requestTenantID(c), id); err != nil {
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

var writeSensitiveWordError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrSensitiveWordNotFound, status: http.StatusNotFound, errorCode: "sensitive_word_not_found"},
)
