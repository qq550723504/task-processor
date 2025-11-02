package management

import (
	"fmt"
	"task-processor/common/config"
	"task-processor/common/management/api"

	"github.com/sirupsen/logrus"
)

// Client 通用管理系统客户端
type Client struct {
	manager *ManagementManager
}

// NewClient 创建管理系统客户端
func NewClient(cfg *config.Config) *Client {
	baseURL := cfg.Management.BaseURL
	if baseURL == "" {
		baseURL = "http://getway.linkcloudai.com"
	}

	return &Client{
		manager: NewManagementManager(baseURL),
	}
}

// SetUserToken 设置用户访问令牌
func (c *Client) SetUserToken(accessToken, tenantID string) {
	c.manager.SetUserToken(accessToken, tenantID)
}

// GetUserToken 获取用户访问令牌
func (c *Client) GetUserToken() (string, string) {
	return c.manager.GetUserToken()
}

// HasValidToken 检查是否有有效的令牌
func (c *Client) HasValidToken() bool {
	return c.manager.HasValidToken()
}

// GetToken 获取当前访问令牌
func (c *Client) GetToken() (string, error) {
	return c.manager.GetToken()
}

// IsAuthenticated 检查是否已认证
func (c *Client) IsAuthenticated() bool {
	return c.manager.IsAuthenticated()
}

// TaskAPIData 任务API数据结构 - 兼容旧接口
type TaskAPIData = api.ProductImportTaskRespDTO

// ===== 导入任务相关API =====

// GetPendingTasks 获取待处理任务
func (c *Client) GetPendingTasks(maxTasks int, userID int64, storeIDs []int64, platform string) ([]TaskAPIData, error) {
	tasks, err := c.manager.ImportTask.GetPendingAndRetryTasks(maxTasks, userID, storeIDs, platform)
	if err != nil {
		return nil, fmt.Errorf("获取待处理任务失败: %w", err)
	}

	logrus.Infof("成功获取 %d 个待处理任务 (平台: %s)", len(tasks), platform)
	return tasks, nil
}

// UpdateTaskStatus 更新任务状态
func (c *Client) UpdateTaskStatus(taskID int64, status int) error {
	req := &api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: int16(status),
	}

	err := c.manager.ImportTask.UpdateTaskStatus(req)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	logrus.Infof("✅ 任务状态已更新: TaskID=%d, Status=%d", taskID, status)
	return nil
}

// UpdateTaskStatusWithError 更新任务状态并包含错误信息
func (c *Client) UpdateTaskStatusWithError(taskID int64, status int, errorMessage string) error {
	req := &api.ProductImportTaskUpdateReqDTO{
		ID:           taskID,
		Status:       int16(status),
		ErrorMessage: errorMessage,
	}

	err := c.manager.ImportTask.UpdateTaskStatus(req)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	logrus.Infof("✅ 任务状态已更新: TaskID=%d, Status=%d, Error=%s", taskID, status, errorMessage)
	return nil
}

// ===== 店铺相关API =====

// GetStore 通过店铺ID获取店铺信息
func (c *Client) GetStore(id int64) (*api.StoreRespDTO, error) {
	store, err := c.manager.Store.GetStore(id)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}

	logrus.Debugf("获取店铺信息成功: ID=%d, Name=%s", store.ID, store.Name)
	return store, nil
}

// GetStoreCookie 通过店铺ID获取用户Cookie
func (c *Client) GetStoreCookie(id int64) (string, error) {
	cookie, err := c.manager.Store.GetStoreCookie(id)
	if err != nil {
		return "", fmt.Errorf("获取店铺Cookie失败: %w", err)
	}

	logrus.Debugf("获取店铺Cookie成功: StoreID=%d", id)
	return cookie, nil
}

// UpdateStoreId 修改店铺的StoreID
func (c *Client) UpdateStoreId(id int64, storeID string) (bool, error) {
	req := &api.StoreIdUpdateReqDTO{
		ID:      id,
		StoreID: storeID,
	}

	success, err := c.manager.Store.UpdateStoreId(req)
	if err != nil {
		return false, fmt.Errorf("更新店铺ID失败: %w", err)
	}

	logrus.Infof("更新店铺ID: ID=%d, StoreID=%s, Success=%t", id, storeID, success)
	return success, nil
}

// UpdateStoreStatus 更新店铺状态
func (c *Client) UpdateStoreStatus(id int64, status int16) (bool, error) {
	req := &api.StoreStatusUpdateReqDTO{
		ID:     id,
		Status: status,
	}

	success, err := c.manager.Store.UpdateStoreStatus(req)
	if err != nil {
		return false, fmt.Errorf("更新店铺状态失败: %w", err)
	}

	logrus.Infof("更新店铺状态: ID=%d, Status=%d, Success=%t", id, status, success)
	return success, nil
}

// CreateProductImportMapping 创建产品导入映射关系
func (c *Client) CreateProductImportMapping(req *api.ProductImportMappingCreateReqDTO) (int64, error) {
	id, err := c.manager.Store.CreateProductImportMapping(req)
	if err != nil {
		return 0, fmt.Errorf("创建产品导入映射关系失败: %w", err)
	}

	logrus.Infof("创建产品导入映射关系成功: ID=%d, ProductID=%s", id, req.ProductId)
	return id, nil
}

// GetProductImportMappingByPlatformProductId 通过平台产品ID获取产品导入映射关系
func (c *Client) GetProductImportMappingByPlatformProductId(platformProductId string) (*api.ProductImportMappingRespDTO, error) {
	req := &api.ProductImportMappingGetReqDTO{
		PlatformProductId: platformProductId,
	}

	mapping, err := c.manager.Store.GetProductImportMappingByPlatformProductId(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品导入映射关系失败: %w", err)
	}

	logrus.Debugf("获取产品导入映射关系成功: PlatformProductID=%s", platformProductId)
	return mapping, nil
}

// DeleteStoreCookie 通过店铺ID删除用户Cookie
func (c *Client) DeleteStoreCookie(id int64) (bool, error) {
	success, err := c.manager.Store.DeleteStoreCookie(id)
	if err != nil {
		return false, fmt.Errorf("删除店铺Cookie失败: %w", err)
	}

	logrus.Infof("删除店铺Cookie: StoreID=%d, Success=%t", id, success)
	return success, nil
}

// SetStorePauseStatus 设置店铺任务暂停状态
func (c *Client) SetStorePauseStatus(id int64, pause bool) (bool, error) {
	success, err := c.manager.Store.SetStorePauseStatus(id, pause)
	if err != nil {
		return false, fmt.Errorf("设置店铺暂停状态失败: %w", err)
	}

	logrus.Infof("设置店铺暂停状态: StoreID=%d, Pause=%t, Success=%t", id, pause, success)
	return success, nil
}

// ===== 原始JSON数据相关API =====

// GetRawJsonData 获取原始JSON数据
func (c *Client) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	data, err := c.manager.RawJsonData.GetRawJsonData(req)
	if err != nil {
		return nil, fmt.Errorf("获取原始JSON数据失败: %w", err)
	}

	logrus.Debugf("获取原始JSON数据成功: ProductID=%s, Platform=%s", req.ProductID, req.Platform)
	return data, nil
}

// ConfirmProductVariants 确认产品变体数据
func (c *Client) ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error) {
	confirmed, err := c.manager.RawJsonData.ConfirmProductVariants(req)
	if err != nil {
		return false, fmt.Errorf("确认产品变体数据失败: %w", err)
	}

	logrus.Infof("确认产品变体数据: ProductID=%s, Confirmed=%t", req.ProductID, confirmed)
	return confirmed, nil
}

// CreateRawJsonData 创建原始JSON数据（提交到服务器缓存）
func (c *Client) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	id, err := c.manager.RawJsonData.CreateRawJsonData(req)
	if err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}

	logrus.Infof("创建原始JSON数据成功: ID=%d, ProductID=%s", id, req.ProductID)
	return id, nil
}

// ===== 规则相关API =====

// GetPricingRule 获取自动核价规则
func (c *Client) GetPricingRule(req *api.PricingRuleReqDTO) (*api.PricingRuleRespDTO, error) {
	rule, err := c.manager.PricingRule.GetPricingRule(req)
	if err != nil {
		return nil, fmt.Errorf("获取自动核价规则失败: %w", err)
	}

	logrus.Debugf("获取自动核价规则成功: RuleID=%d, Name=%s", rule.ID, rule.Name)
	return rule, nil
}

// GetProfitRule 获取利润规则
func (c *Client) GetProfitRule(req *api.ProfitRuleReqDTO) (*api.ProfitRuleRespDTO, error) {
	rule, err := c.manager.ProfitRule.GetProfitRule(req)
	if err != nil {
		return nil, fmt.Errorf("获取利润规则失败: %w", err)
	}

	logrus.Debugf("获取利润规则成功: RuleID=%d, Name=%s", rule.ID, rule.Name)
	return rule, nil
}

// GetFilterRule 获取过滤规则
func (c *Client) GetFilterRule(req *api.FilterRuleReqDTO) (*[]api.FilterRuleRespDTO, error) {
	rules, err := c.manager.FilterRule.GetFilterRule(req)
	if err != nil {
		return nil, fmt.Errorf("获取过滤规则失败: %w", err)
	}

	logrus.Debugf("获取过滤规则成功: 规则数量=%d", len(*rules))
	return rules, nil
}

// ===== 兼容性方法 =====

// CompleteTask 完成任务 - 兼容旧接口
func (c *Client) CompleteTask(taskID int64, result map[string]any) error {
	return c.UpdateTaskStatus(taskID, 2) // 2 = 完成状态
}

// FailTask 标记任务失败 - 兼容旧接口
func (c *Client) FailTask(taskID int64, errorMsg string) error {
	return c.UpdateTaskStatusWithError(taskID, 3, errorMsg) // 3 = 失败状态
}

// GetTaskStatus 获取任务状态 - 兼容旧接口（需要实现具体逻辑）
func (c *Client) GetTaskStatus(taskID int64) (int, error) {
	// 这个方法需要后端提供对应的API
	// 目前返回一个占位符实现
	logrus.Warnf("GetTaskStatus方法需要后端API支持: TaskID=%d", taskID)
	return -1, fmt.Errorf("GetTaskStatus方法暂未实现，需要后端API支持")
}
