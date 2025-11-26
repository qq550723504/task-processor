package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

type TemplateQueryHandler struct {
	logger *logrus.Entry
}

// TemplateQueryRequest 模板查询请求结构体
type TemplateQueryRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
}

// ValueUnit 值单位结构体
type ValueUnit struct {
	ValueUnit   string `json:"value_unit"`
	ValueUnitID string `json:"value_unit_id"`
}

// Group 分组结构体
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SubGroup 子分组结构体
type SubGroup struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PropertyValue 属性值结构体
type PropertyValue struct {
	VID        int       `json:"vid"`
	Value      string    `json:"value"`
	Group      *Group    `json:"group,omitempty"`
	SubGroup   *SubGroup `json:"sub_group,omitempty"`
	SpecID     string    `json:"spec_id,omitempty"`
	ExtendInfo string    `json:"extend_info,omitempty"`
	ParentVIDs []int     `json:"parent_vids,omitempty"`
}

// ShowCondition 显示条件结构体
type ShowCondition struct {
	ParentRefPID int   `json:"parent_ref_pid"`
	ParentVIDs   []int `json:"parent_vids"`
}

// GroupValues 分组值结构体
type GroupValues struct {
	Name           string                    `json:"name"`
	Values         []PropertyValue           `json:"values"`
	SubGroupValues map[string]SubGroupValues `json:"sub_group_values,omitempty"`
}

// SubGroupValues 子分组值结构体
type SubGroupValues struct {
	SubGroupName string          `json:"sub_group_name"`
	Values       []PropertyValue `json:"values"`
}

// TemplatePropertyValueParent 模板属性值父级结构体
type TemplatePropertyValueParent struct {
	VIDs       []int `json:"vids"`
	ParentVIDs []int `json:"parent_vids"`
}

// GoodsProperty 商品属性结构体
type GoodsProperty struct {
	PID int `json:"pid"`
	//TemplateModuleID                int                           `json:"template_module_id"`
	TemplatePID                     int                           `json:"template_pid"`
	RefPID                          int                           `json:"ref_pid"`
	Name                            string                        `json:"name"`
	PropertyValueType               int                           `json:"property_value_type"`
	ValueUnit                       []string                      `json:"value_unit"`
	ValueUnitDTOList                []ValueUnit                   `json:"value_unit_dtolist,omitempty"`
	Values                          []PropertyValue               `json:"values,omitempty"`
	ChooseMaxNum                    int                           `json:"choose_max_num"`
	MaxValue                        string                        `json:"max_value"`
	MinValue                        string                        `json:"min_value"`
	ValuePrecision                  int                           `json:"value_precision"`
	ShowCondition                   []ShowCondition               `json:"show_condition,omitempty"`
	Required                        bool                          `json:"required"`
	IsSale                          bool                          `json:"is_sale"`
	Feature                         int                           `json:"feature"`
	PropertyChooseTitle             string                        `json:"property_choose_title,omitempty"`
	NumberInputTitle                string                        `json:"number_input_title,omitempty"`
	ValueRule                       int                           `json:"value_rule"`
	ControlType                     int                           `json:"control_type"`
	ShowType                        int                           `json:"show_type"`
	ParentTemplatePID               int                           `json:"parent_template_pid"`
	TemplatePropertyValueParentList []TemplatePropertyValueParent `json:"template_property_value_parent_list,omitempty"`
}

// GoodsSpecProperty 商品规格属性结构体
type GoodsSpecProperty struct {
	PID               int                    `json:"pid"`
	TemplateModuleID  int                    `json:"template_module_id"`
	TemplatePID       int                    `json:"template_pid"`
	RefPID            int                    `json:"ref_pid"`
	Name              string                 `json:"name"`
	PropertyValueType int                    `json:"property_value_type"`
	ValueUnit         []string               `json:"value_unit"`
	Values            []PropertyValue        `json:"values"`
	Group2Values      map[string]GroupValues `json:"group2_values,omitempty"`
	MaxValue          string                 `json:"max_value"`
	MinValue          string                 `json:"min_value"`
	ValuePrecision    int                    `json:"value_precision"`
	Required          bool                   `json:"required"`
	IsSale            bool                   `json:"is_sale"`
	ParentSpecID      string                 `json:"parent_spec_id,omitempty"`
	MainSale          bool                   `json:"main_sale"`
	Feature           int                    `json:"feature"`
	ControlType       int                    `json:"control_type"`
}

// UserInputParentSpec 用户输入父规格结构体
type UserInputParentSpec struct {
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name"`
}

// PubConfig 发布配置结构体
type PubConfig struct {
	Currency                  string `json:"currency"`
	CurrencySymbol            string `json:"currency_symbol"`
	VolumeUnit                string `json:"volume_unit"`
	WeightUnit                string `json:"weight_unit"`
	IsSymbolAfterPrice        bool   `json:"is_symbol_after_price"`
	RetailPriceCurrency       string `json:"retail_price_currency"`
	RetailPriceCurrencySymbol string `json:"retail_price_currency_symbol"`
	RetailIsSymbolAfterPrice  bool   `json:"retail_is_symbol_after_price"`
}

// TemplateInfo 模板信息结构体
type TemplateInfo struct {
	TemplateID          int                 `json:"template_id"`
	GoodsProperties     []GoodsProperty     `json:"goods_properties"`
	GoodsSpecProperties []GoodsSpecProperty `json:"goods_spec_properties"`
}

// TemplateQueryResult 模板查询结果结构体
type TemplateQueryResult struct {
	InputMaxSpecNum         int                   `json:"input_max_spec_num"`
	SingleSpecValueNum      int                   `json:"single_spec_value_num"`
	AuthenticationLinkURL   string                `json:"authentication_link_url"`
	NeedMultiOriginRegion   bool                  `json:"need_multi_origin_region"`
	PubConfig               PubConfig             `json:"pub_config"`
	UserInputParentSpecList []UserInputParentSpec `json:"user_input_parent_spec_list"`
	TemplateInfo            TemplateInfo          `json:"template_info"`
}

// TemplateQueryResponse 模板查询响应结构体
type TemplateQueryResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    TemplateQueryResult `json:"result"`
}

func NewTemplateQueryHandler() *TemplateQueryHandler {
	return &TemplateQueryHandler{
		logger: logrus.WithField("handler", "TemplateQueryHandler"),
	}
}

func (h *TemplateQueryHandler) Name() string {
	return "模板查询处理器"
}

func (h *TemplateQueryHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始执行模板查询处理器")

	// 检查API客户端
	if ctx.APIClient == nil {
		h.logger.Error("API客户端未初始化，跳过模板查询 - 这会导致后续SKU构建失败")
		return nil
	}

	h.logger.Info("API客户端已初始化，继续模板查询")

	// 从上下文获取必要的参数
	request := h.buildTemplateQueryRequest(ctx)

	// 发送模板查询请求
	err := h.queryTemplate(ctx, request)
	if err != nil {
		h.logger.WithError(err).Error("模板查询失败")
		return err
	}

	h.logger.Info("模板查询完成")
	return nil
}

// buildTemplateQueryRequest 构建模板查询请求参数
func (h *TemplateQueryHandler) buildTemplateQueryRequest(ctx *pipeline.TaskContext) TemplateQueryRequest {
	request := TemplateQueryRequest{
		ClickType: "8",
	}

	// 检查TemuProduct是否存在
	if ctx.TemuProduct == nil {
		h.logger.Warn("TemuProduct为空，无法获取模板查询参数")
		return request
	}

	h.logger.Debugf("TemuProduct存在，开始提取参数")

	// 从上下文中获取实际的参数值
	if ctx.TemuProduct != nil {
		if ctx.TemuProduct.GoodsBasic.ListingCommitID != "" {
			request.ListingCommitID = ctx.TemuProduct.GoodsBasic.ListingCommitID
		}
		if ctx.TemuProduct.GoodsBasic.GoodsCommitID != "" {
			request.GoodsCommitID = ctx.TemuProduct.GoodsBasic.GoodsCommitID
		}
		if ctx.TemuProduct.GoodsBasic.GoodsID != "" {
			request.GoodsID = ctx.TemuProduct.GoodsBasic.GoodsID
		}
		if ctx.TemuProduct.GoodsBasic.CatID > 0 {
			request.CatID = ctx.TemuProduct.GoodsBasic.CatID
		}
		if ctx.TemuProduct.GoodsBasic.ListingCommitVersion != "" {
			request.ListingCommitVersion = ctx.TemuProduct.GoodsBasic.ListingCommitVersion
		}
	}

	return request
}

// queryTemplate 发送模板查询请求到TEMU API
func (h *TemplateQueryHandler) queryTemplate(ctx *pipeline.TaskContext, request TemplateQueryRequest) error {
	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_template",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": request,
	}

	// 发送请求
	response := &TemplateQueryResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}

	// 检查响应是否成功
	if !response.Success {
		return fmt.Errorf("模板查询失败: error_code=%d", response.ErrorCode)
	}

	// 将模板信息存储到上下文中，供后续处理器使用
	ctx.SetData("template_info", response.Result.TemplateInfo)
	ctx.SetData("user_input_parent_spec_list", response.Result.UserInputParentSpecList)
	ctx.SetData("input_max_spec_num", response.Result.InputMaxSpecNum)
	ctx.SetData("single_spec_value_num", response.Result.SingleSpecValueNum)

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":      request.ListingCommitID,
		"goodsCommitID":        request.GoodsCommitID,
		"catID":                request.CatID,
		"templateID":           response.Result.TemplateInfo.TemplateID,
		"goodsPropertiesCount": len(response.Result.TemplateInfo.GoodsProperties),
		"specPropertiesCount":  len(response.Result.TemplateInfo.GoodsSpecProperties),
		"success":              response.Success,
	}).Info("模板查询成功，已存储到上下文")

	// 额外调试：验证数据确实被存储了
	if storedData, exists := ctx.GetData("template_info"); exists {
		if storedTemplateInfo, ok := storedData.(TemplateInfo); ok {
			h.logger.Infof("验证：模板信息已成功存储，规格属性数量: %d", len(storedTemplateInfo.GoodsSpecProperties))
		} else {
			h.logger.Errorf("验证失败：存储的数据类型不正确，类型: %T", storedData)
		}
	} else {
		h.logger.Error("验证失败：数据未能正确存储到上下文")
	}

	return nil
}

// GetTemplateInfoFromContext 从上下文中获取模板信息
func GetTemplateInfoFromContext(ctx *pipeline.TaskContext) (*TemplateInfo, bool) {
	if data, exists := ctx.GetData("template_info"); exists {
		if templateInfo, ok := data.(TemplateInfo); ok {
			return &templateInfo, true
		}
	}
	return nil, false
}

// GetInputMaxSpecNumFromContext 从上下文中获取最大规格数量
func GetInputMaxSpecNumFromContext(ctx *pipeline.TaskContext) (int, bool) {
	if data, exists := ctx.GetData("input_max_spec_num"); exists {
		if maxSpecNum, ok := data.(int); ok {
			return maxSpecNum, true
		}
	}
	return 0, false
}

// GetSingleSpecValueNumFromContext 从上下文中获取单规格值数量
func GetSingleSpecValueNumFromContext(ctx *pipeline.TaskContext) (int, bool) {
	if data, exists := ctx.GetData("single_spec_value_num"); exists {
		if singleSpecNum, ok := data.(int); ok {
			return singleSpecNum, true
		}
	}
	return 0, false
}

// GetUserInputParentSpecListFromContext 从上下文中获取用户输入父规格列表
func GetUserInputParentSpecListFromContext(ctx *pipeline.TaskContext) ([]UserInputParentSpec, bool) {
	if data, exists := ctx.GetData("user_input_parent_spec_list"); exists {
		if userInputSpecs, ok := data.([]UserInputParentSpec); ok {
			return userInputSpecs, true
		}
	}
	return nil, false
}
