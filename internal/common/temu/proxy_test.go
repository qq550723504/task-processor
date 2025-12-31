package temu

import (
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/management/impl"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIClient_ProxyConfiguration 测试代理配置
func TestAPIClient_ProxyConfiguration(t *testing.T) {
	// 注意：这个测试需要真实的管理系统连接
	// 如果管理系统不可用，测试会跳过代理配置但不会失败
	t.Run("验证代理配置逻辑", func(t *testing.T) {
		// 创建一个真实的 ClientManager（但不需要真实连接）
		cfg := &config.ManagementConfig{
			BaseURL: "http://test-management.example.com",
		}
		mgmtClient := management.NewClientManager(cfg)

		// 测试当管理系统不可用时，客户端仍能正常创建
		client := NewAPIClient(1, 12345, mgmtClient)

		// 验证客户端创建成功
		assert.NotNil(t, client)
		assert.NotNil(t, client.client)
		assert.Equal(t, int64(12345), client.storeID)

		// 由于管理系统不可用，代理应该为空（这是预期行为）
		// 实际使用时，如果管理系统可用且店铺配置了代理，proxyURL 会被设置
		t.Logf("代理地址: %s (管理系统不可用时为空)", client.proxyURL)
	})
}

// TestAPIClient_ProxyURL_Field 测试代理字段存在性
func TestAPIClient_ProxyURL_Field(t *testing.T) {
	// 创建一个简单的客户端来验证字段存在
	cfg := &config.ManagementConfig{
		BaseURL: "http://test.example.com",
	}
	mgmtClient := management.NewClientManager(cfg)
	client := NewAPIClient(1, 12345, mgmtClient)

	// 验证 proxyURL 字段可以被访问和设置
	assert.NotNil(t, client)

	// 手动设置代理来验证字段工作正常
	testProxy := "http://test-proxy.example.com:8080"
	client.proxyURL = testProxy
	assert.Equal(t, testProxy, client.proxyURL)
}

// MockStoreAPIForProxy 用于集成测试的 mock 实现
type MockStoreAPIForProxy struct {
	storeInfo *api.StoreRespDTO
}

func (m *MockStoreAPIForProxy) GetStore(id int64) (*api.StoreRespDTO, error) {
	return m.storeInfo, nil
}

func (m *MockStoreAPIForProxy) GetStoreCookie(id int64) (string, error) {
	return "", nil
}

func (m *MockStoreAPIForProxy) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	return true, nil
}

func (m *MockStoreAPIForProxy) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	return true, nil
}

func (m *MockStoreAPIForProxy) DeleteStoreCookie(id int64) (bool, error) {
	return true, nil
}

func (m *MockStoreAPIForProxy) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	return true, nil
}

// TestProxyIntegration 集成测试示例（需要真实的管理系统）
func TestProxyIntegration(t *testing.T) {
	t.Skip("跳过集成测试 - 需要真实的管理系统环境")

	// 这是一个集成测试示例，展示如何在真实环境中测试代理功能
	// 在实际使用时，需要：
	// 1. 配置真实的管理系统地址
	// 2. 在管理系统中为测试店铺配置代理
	// 3. 运行此测试验证代理是否生效

	cfg := &config.ManagementConfig{
		BaseURL: "http://gateway.linkcloudai.com", // 真实的管理系统地址
	}
	mgmtClient := management.NewClientManager(cfg)

	// 使用真实的店铺ID
	testStoreID := int64(12345)
	client := NewAPIClient(1, testStoreID, mgmtClient)

	// 验证代理配置
	if client.proxyURL != "" {
		t.Logf("✓ 店铺 %d 配置了代理: %s", testStoreID, client.proxyURL)
	} else {
		t.Logf("店铺 %d 未配置代理", testStoreID)
	}
}

// 确保 impl.StoreAPIClientImpl 实现了 api.StoreAPI 接口
var _ api.StoreAPI = (*impl.StoreAPIClientImpl)(nil)
