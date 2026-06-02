package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/shein/generationtopics"
)

type GenerationTopicOverrideHandler struct {
	repo GenerationTopicOverrideRepository
}

func NewGenerationTopicOverrideHandler(repo GenerationTopicOverrideRepository) *GenerationTopicOverrideHandler {
	return &GenerationTopicOverrideHandler{repo: repo}
}

func (h *GenerationTopicOverrideHandler) ListGenerationTopicOverrides(c *gin.Context) {
	scope := requestListScope(c)
	query := &GenerationTopicOverrideQuery{
		TenantID:    scope.TenantID,
		OwnerUserID: scope.OwnerUserID,
		Page:        scope.Page,
		PageSize:    scope.PageSize,
		Platform:    strings.TrimSpace(c.Query("platform")),
		TopicKey:    strings.TrimSpace(c.Query("topic_key")),
		Remark:      strings.TrimSpace(c.Query("remark")),
	}
	var ok bool
	query.Status, ok = queryInt16PtrStrict(c, "status", "invalid_status")
	if !ok {
		return
	}
	page, err := h.repo.ListGenerationTopicOverrides(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "generation_topic_override_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *GenerationTopicOverrideHandler) GetGenerationTopicOverride(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.repo.GetGenerationTopicOverride(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeGenerationTopicOverrideError(c, err, "generation_topic_override_get_failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *GenerationTopicOverrideHandler) CreateGenerationTopicOverride(c *gin.Context) {
	var req GenerationTopicOverride
	if !bindAndValidateJSON(c, &req, "invalid_generation_topic_override", func(value *GenerationTopicOverride) {
		value.TenantID = requestTenantID(c)
	}, validateGenerationTopicOverride) {
		return
	}
	item, err := h.repo.CreateGenerationTopicOverride(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "generation_topic_override_create_failed", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *GenerationTopicOverrideHandler) UpdateGenerationTopicOverride(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req GenerationTopicOverride
	if !bindAndValidateJSON(c, &req, "invalid_generation_topic_override", func(value *GenerationTopicOverride) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateGenerationTopicOverride) {
		return
	}
	item, err := h.repo.UpdateGenerationTopicOverride(requestIdentityContext(c), &req)
	if err != nil {
		writeGenerationTopicOverrideError(c, err, "generation_topic_override_update_failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *GenerationTopicOverrideHandler) UpdateGenerationTopicOverrideStatus(c *gin.Context) {
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
	item, err := h.repo.UpdateGenerationTopicOverrideStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeGenerationTopicOverrideError(c, err, "generation_topic_override_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *GenerationTopicOverrideHandler) DeleteGenerationTopicOverride(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteGenerationTopicOverride(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeGenerationTopicOverrideError(c, err, "generation_topic_override_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateGenerationTopicOverride(item *GenerationTopicOverride) error {
	switch {
	case item.TenantID <= 0:
		return errors.New("tenant id is required")
	case generationtopics.NormalizeKey(item.Platform) != "shein":
		return errors.New("platform must be shein")
	case strings.TrimSpace(item.TopicKey) == "":
		return errors.New("topic key is required")
	case len(strings.TrimSpace(item.TopicKey)) > 64:
		return errors.New("topic key is too long")
	}

	normalizedTopicKey := generationtopics.NormalizeKey(item.TopicKey)
	if _, unknown := generationtopics.ResolveSheinTopicKeys([]string{normalizedTopicKey}); len(unknown) > 0 {
		return errors.New("topic key must exist in shein topic catalog")
	}

	item.Platform = generationtopics.NormalizeKey(item.Platform)
	item.TopicKey = normalizedTopicKey
	item.AdditionalPromptDirectives = normalizeStringList(item.AdditionalPromptDirectives)
	item.AdditionalLexiconByLanguage = normalizeLexiconMap(item.AdditionalLexiconByLanguage)
	item.Remark = strings.TrimSpace(item.Remark)
	return nil
}

var writeGenerationTopicOverrideError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrGenerationTopicOverrideNotFound, status: http.StatusNotFound, errorCode: "generation_topic_override_not_found"},
)
