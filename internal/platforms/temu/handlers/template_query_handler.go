package handlers

import (
	"fmt"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

type TemplateQueryHandler struct {
	logger *logrus.Entry
}

func NewTemplateQueryHandler() *TemplateQueryHandler {
	return &TemplateQueryHandler{
		logger: logrus.WithField("handler", "TemplateQueryHandler"),
	}
}

func (h *TemplateQueryHandler) Name() string {
	return "模板查询处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *TemplateQueryHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始执行模板查询处理器")

	// 检查API客户端
	if temuCtx.APIClient == nil {
		h.logger.Error("API客户端未初始化，跳过模板查询 - 这会导致后续SKU构建失败")
		return nil
	}

	h.logger.Info("API客户端已初始化，继续模板查询")

	// 从上下文获取必要的参数
	request := h.buildTemplateQueryRequest(temuCtx)

	// 发送模板查询请求
	err := h.queryTemplate(temuCtx, request)
	if err != nil {
		h.logger.WithError(err).Error("模板查询失败")
		return err
	}

	h.logger.Info("模板查询完成")
	return nil
}

// buildTemplateQueryRequest 构建模板查询请求参数
func (h *TemplateQueryHandler) buildTemplateQueryRequest(temuCtx *temucontext.TemuTaskContext) types.TemplateQueryRequest {
	request := types.TemplateQueryRequest{
		ClickType: "8",
	}

	// 从强类型上下文中获取TemuProduct数据
	if temuCtx.TemuProduct != nil {
		h.logger.Debugf("TemuProduct存在，开始提取参数")

		temuProduct := temuCtx.TemuProduct
		if temuProduct.GoodsBasic.ListingCommitID != "" {
			request.ListingCommitID = temuProduct.GoodsBasic.ListingCommitID
		}
		if temuProduct.GoodsBasic.GoodsCommitID != "" {
			request.GoodsCommitID = temuProduct.GoodsBasic.GoodsCommitID
		}
		if temuProduct.GoodsBasic.GoodsID != "" {
			request.GoodsID = temuProduct.GoodsBasic.GoodsID
		}
		if temuProduct.GoodsBasic.CatID > 0 {
			request.CatID = temuProduct.GoodsBasic.CatID
		}
		if temuProduct.GoodsBasic.ListingCommitVersion != "" {
			request.ListingCommitVersion = temuProduct.GoodsBasic.ListingCommitVersion
		}
	} else {
		h.logger.Warn("TemuProduct为空，无法获取模板查询参数")
	}

	return request
}

// queryTemplate 发送模板查询请求到TEMU API
func (h *TemplateQueryHandler) queryTemplate(temuCtx *temucontext.TemuTaskContext, request types.TemplateQueryRequest) error {
	// 获取API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 创建QueryAPI
	queryAPI := api.NewQueryAPI(temuCtx.APIClient, h.logger)

	// 构造请求（需要转换类型）
	apiRequest := &models.TemplateQueryRequest{
		CatID: request.CatID,
	}

	// 调用API查询模板
	response, err := queryAPI.QueryTemplate(apiRequest)
	if err != nil {
		return fmt.Errorf("模板查询失败: %v", err)
	}

	// 将模板信息存储到强类型字段中（需要根据实际的types结构进行转换）
	// 这里需要根据实际的types.TemplateQueryResponse结构来适配
	h.logger.WithFields(logrus.Fields{
		"templateCount": len(response.Result.Templates),
	}).Info("模板查询成功")

	return nil
}

// GetTemplateInfoFromContext 从强类型上下文中获取模板信息
func GetTemplateInfoFromContext(temuCtx *temucontext.TemuTaskContext) (*types.TemplateInfo, bool) {
	if temuCtx.TemplateInfo != nil {
		if templateInfo, ok := temuCtx.TemplateInfo.(types.TemplateInfo); ok {
			return &templateInfo, true
		}
	}
	return nil, false
}

// GetInputMaxSpecNumFromContext 从强类型上下文中获取最大规格数量
func GetInputMaxSpecNumFromContext(temuCtx *temucontext.TemuTaskContext) (int, bool) {
	return temuCtx.InputMaxSpecNum, temuCtx.InputMaxSpecNum > 0
}

// GetSingleSpecValueNumFromContext 从强类型上下文中获取单规格值数量
func GetSingleSpecValueNumFromContext(temuCtx *temucontext.TemuTaskContext) (int, bool) {
	return temuCtx.SingleSpecValueNum, temuCtx.SingleSpecValueNum > 0
}

// GetUserInputParentSpecListFromContext 从强类型上下文中获取用户输入父规格列表
func GetUserInputParentSpecListFromContext(temuCtx *temucontext.TemuTaskContext) ([]types.UserInputParentSpec, bool) {
	if temuCtx.UserInputParentSpecList != nil {
		if userInputSpecs, ok := temuCtx.UserInputParentSpecList.([]types.UserInputParentSpec); ok {
			return userInputSpecs, true
		}
	}
	return nil, false
}
