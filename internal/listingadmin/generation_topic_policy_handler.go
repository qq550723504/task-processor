package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type GenerationTopicPolicyHandler struct {
	repo GenerationTopicPolicyRepository
}

func NewGenerationTopicPolicyHandler(repo GenerationTopicPolicyRepository) *GenerationTopicPolicyHandler {
	return &GenerationTopicPolicyHandler{repo: repo}
}

func (h *GenerationTopicPolicyHandler) ListGenerationTopicPolicies(c *gin.Context) {
	query := applyListQueryScope(&GenerationTopicPolicyQuery{
		Platform: strings.TrimSpace(c.Query("platform")),
		TopicKey: strings.TrimSpace(c.Query("topic_key")),
		Remark:   strings.TrimSpace(c.Query("remark")),
	}, requestListScope(c))
	var ok bool
	query.Status, ok = queryInt16PtrStrict(c, "status", "invalid_status")
	if !ok {
		return
	}
	page, err := h.repo.ListGenerationTopicPolicies(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "generation_topic_policy_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *GenerationTopicPolicyHandler) GetGenerationTopicPolicy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	policy, err := h.repo.GetGenerationTopicPolicy(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeGenerationTopicPolicyError(c, err, "generation_topic_policy_get_failed")
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *GenerationTopicPolicyHandler) CreateGenerationTopicPolicy(c *gin.Context) {
	var req GenerationTopicPolicy
	if !bindAndValidateJSON(c, &req, "invalid_generation_topic_policy", func(value *GenerationTopicPolicy) {
		value.TenantID = requestTenantID(c)
	}, validateGenerationTopicPolicy) {
		return
	}
	policy, err := h.repo.CreateGenerationTopicPolicy(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "generation_topic_policy_create_failed", err)
		return
	}
	c.JSON(http.StatusCreated, policy)
}

func (h *GenerationTopicPolicyHandler) UpdateGenerationTopicPolicy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req GenerationTopicPolicy
	if !bindAndValidateJSON(c, &req, "invalid_generation_topic_policy", func(value *GenerationTopicPolicy) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateGenerationTopicPolicy) {
		return
	}
	policy, err := h.repo.UpdateGenerationTopicPolicy(requestIdentityContext(c), &req)
	if err != nil {
		writeGenerationTopicPolicyError(c, err, "generation_topic_policy_update_failed")
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *GenerationTopicPolicyHandler) UpdateGenerationTopicPolicyStatus(c *gin.Context) {
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
	policy, err := h.repo.UpdateGenerationTopicPolicyStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeGenerationTopicPolicyError(c, err, "generation_topic_policy_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *GenerationTopicPolicyHandler) DeleteGenerationTopicPolicy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteGenerationTopicPolicy(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeGenerationTopicPolicyError(c, err, "generation_topic_policy_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateGenerationTopicPolicy(policy *GenerationTopicPolicy) error {
	switch {
	case policy.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(policy.Platform) == "":
		return errors.New("platform is required")
	case strings.TrimSpace(policy.TopicKey) == "":
		return errors.New("topic key is required")
	case len(strings.TrimSpace(policy.TopicKey)) > 64:
		return errors.New("topic key is too long")
	}
	return nil
}

var writeGenerationTopicPolicyError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrGenerationTopicPolicyNotFound, status: http.StatusNotFound, errorCode: "generation_topic_policy_not_found"},
)
