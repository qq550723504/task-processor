package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 产品提交接口
	productSubmitEndpoint = "/mms/marigold/edit/commit/submit"
	// 创建新提交接口
	createCommitEndpoint = "/mms/marigold/edit/commit/create_new"
)

// SubmitAPI 提交API管理器
type SubmitAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewSubmitAPI 创建新的提交API管理器
func NewSubmitAPI(client client.APIClientInterface, logger *logrus.Entry) *SubmitAPI {
	return &SubmitAPI{
		client: client,
		logger: logger,
	}
}

// buildStandardHeaders 构建标准请求头
func (s *SubmitAPI) buildStandardHeaders() map[string]string {
	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"
	return headers
}

// temuRequest 通用的TEMU API请求方法
func (s *SubmitAPI) temuRequest(
	endpoint string,
	request interface{},
	result interface{},
	actionName string,
) error {
	s.logger.Infof("开始%s", actionName)

	if request == nil {
		return fmt.Errorf("%s请求不能为空", actionName)
	}

	headers := s.buildStandardHeaders()

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     endpoint,
		"headers": headers,
		"body":    request,
	}

	if err := s.client.SendTEMURequest(apiReq, result); err != nil {
		s.logger.WithError(err).Errorf("%s请求失败", actionName)
		return fmt.Errorf("%s请求失败: %w", actionName, err)
	}

	return nil
}

// validateResponse 验证响应结果
func (s *SubmitAPI) validateResponse(result interface{}, actionName string) error {
	// 使用类型断言检查Success字段
	type SuccessChecker interface {
		IsSuccess() bool
		GetErrorCode() int
		GetMessage() string
	}

	if checker, ok := result.(SuccessChecker); ok {
		if !checker.IsSuccess() {
			s.logger.Errorf("%s失败: errorCode=%d, message=%s",
				actionName, checker.GetErrorCode(), checker.GetMessage())
			return fmt.Errorf("%s失败: errorCode=%d, message=%s",
				actionName, checker.GetErrorCode(), checker.GetMessage())
		}
	}

	return nil
}

// SubmitProduct 提交产品
func (s *SubmitAPI) SubmitProduct(request *models.ProductSubmitRequest) (*models.ProductSubmitResponse, error) {
	var result models.ProductSubmitResponse

	if err := s.temuRequest(productSubmitEndpoint, request, &result, "产品提交"); err != nil {
		return nil, err
	}

	if !result.Success {
		s.logger.Errorf("产品提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
		return nil, fmt.Errorf("产品提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}

	s.logger.Infof("产品提交成功: ListingCommitID=%s",
		func() string {
			if result.Result != nil {
				return result.Result.ListingCommitID
			}
			return "未返回"
		}())

	return &result, nil
}

// CreateCommit 创建新的提交
func (s *SubmitAPI) CreateCommit(request *models.CreateCommitRequest) (*models.CreateCommitResponse, error) {
	var result models.CreateCommitResponse

	if err := s.temuRequest(createCommitEndpoint, request, &result, "创建提交"); err != nil {
		return nil, err
	}

	if !result.Success {
		s.logger.Errorf("创建提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
		return nil, fmt.Errorf("创建提交失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}

	s.logger.Infof("创建提交成功: ListingCommitID=%s",
		func() string {
			if result.Result != nil {
				return result.Result.ListingCommitID
			}
			return "未返回"
		}())

	return &result, nil
}
