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

// SubmitProduct 提交产品
func (s *SubmitAPI) SubmitProduct(request *models.ProductSubmitRequest) (*models.ProductSubmitResponse, error) {
	s.logger.Info("开始提交产品")

	if request == nil {
		return nil, fmt.Errorf("提交请求不能为空")
	}

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

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     productSubmitEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.ProductSubmitResponse
	if err := s.client.SendTEMURequest(apiReq, &result); err != nil {
		s.logger.WithError(err).Error("产品提交请求失败")
		return nil, fmt.Errorf("产品提交请求失败: %w", err)
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
	s.logger.Info("开始创建新提交")

	if request == nil {
		return nil, fmt.Errorf("创建请求不能为空")
	}

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

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     createCommitEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.CreateCommitResponse
	if err := s.client.SendTEMURequest(apiReq, &result); err != nil {
		s.logger.WithError(err).Error("创建提交请求失败")
		return nil, fmt.Errorf("创建提交请求失败: %w", err)
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
