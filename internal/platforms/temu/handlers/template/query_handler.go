package template

import (
	"fmt"
	"task-processor/internal/core/logger"
	temuquery "task-processor/internal/platforms/temu/api/query"
	temutemplate "task-processor/internal/platforms/temu/api/template"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

type TemplateQueryHandler struct {
	logger *logrus.Entry
}

func NewTemplateQueryHandler() *TemplateQueryHandler {
	return &TemplateQueryHandler{
		logger: logger.GetGlobalLogger("temu.handlers.template_query").WithField("handler", "TemplateQueryHandler"),
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
func (h *TemplateQueryHandler) buildTemplateQueryRequest(temuCtx *temucontext.TemuTaskContext) temutemplate.TemplateQueryRequest {
	request := temutemplate.TemplateQueryRequest{
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
func (h *TemplateQueryHandler) queryTemplate(temuCtx *temucontext.TemuTaskContext, request temutemplate.TemplateQueryRequest) error {
	// 检查API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 检查QueryAPI服务是否存在
	if temuCtx.QueryAPI == nil {
		return fmt.Errorf("QueryAPI服务未初始化")
	}

	// 类型断言获取QueryAPI实例
	queryAPI, ok := temuCtx.QueryAPI.(*temuquery.API)
	if !ok {
		return fmt.Errorf("QueryAPI类型断言失败")
	}

	// 使用QueryAPI服务发送请求
	response, err := queryAPI.QueryTemplateAdvanced(&request)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}

	// 将模板信息存储到强类型字段中
	temuCtx.TemplateInfo = response.Result.TemplateInfo
	temuCtx.UserInputParentSpecList = response.Result.UserInputParentSpecList
	temuCtx.InputMaxSpecNum = response.Result.InputMaxSpecNum
	temuCtx.SingleSpecValueNum = response.Result.SingleSpecValueNum

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":     request.ListingCommitID,
		"goodsCommitID":       request.GoodsCommitID,
		"catID":               request.CatID,
		"templateID":          response.Result.TemplateInfo.TemplateID,
		"specPropertiesCount": len(response.Result.TemplateInfo.GoodsSpecProperties),
		"success":             response.Success,
	}).Info("模板查询成功，已存储到强类型上下文")

	return nil
}

// GetTemplateInfoFromContext 从强类型上下文中获取模板信息
func GetTemplateInfoFromContext(temuCtx *temucontext.TemuTaskContext) (*temutemplate.TemplateInfo, bool) {
	if temuCtx.TemplateInfo != nil {
		if templateInfo, ok := temuCtx.TemplateInfo.(temutemplate.TemplateInfo); ok {
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
func GetUserInputParentSpecListFromContext(temuCtx *temucontext.TemuTaskContext) ([]temutemplate.UserInputParentSpec, bool) {
	if temuCtx.UserInputParentSpecList != nil {
		if userInputSpecs, ok := temuCtx.UserInputParentSpecList.([]temutemplate.UserInputParentSpec); ok {
			return userInputSpecs, true
		}
	}
	return nil, false
}
