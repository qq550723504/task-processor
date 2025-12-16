// Package amazon 提供Amazon平台管道测试
package amazon

import (
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/internal/service"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestAmazonPipeline 测试完整的Amazon处理管道
func TestAmazonPipeline(t *testing.T) {
	logrus.Info("开始测试Amazon处理管道")

	// 创建服务集合
	services := model.NewServices()
	// 注意：在实际测试中需要设置真实的服务
	// services.SetAPIClient(apiClient)
	// services.SetProductTypeCache(cache)

	// 创建管道构建器
	builder := service.NewPipelineBuilder(services)
	pipeline := builder.BuildAmazonPipeline()

	// 验证管道处理器数量
	expectedHandlers := 11
	actualHandlers := pipeline.GetHandlerCount()

	if actualHandlers != expectedHandlers {
		t.Errorf("期望 %d 个处理器，实际得到 %d 个", expectedHandlers, actualHandlers)
	}

	// 注意：这里只测试管道构建，不执行实际处理
	// 因为需要真实的API客户端和服务
	logrus.Infof("Amazon处理管道构建成功，包含 %d 个处理器", actualHandlers)

	t.Logf("管道测试完成，处理器数量: %d", actualHandlers)
}
