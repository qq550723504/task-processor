// Package pricing 提供通用核价服务的工厂类和初始化方法。
package pricing

import (
	"task-processor/common/management"
	"task-processor/common/pricing/adapter"
	"task-processor/common/pricing/service"

	"github.com/sirupsen/logrus"
)

// 导出适配器类型以便外部使用
type TemuAdapter = adapter.TemuAdapter
type SheinAdapter = adapter.SheinAdapter

// ServiceFactory 核价服务工厂
type ServiceFactory struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewServiceFactory 创建核价服务工厂
func NewServiceFactory(managementClient *management.ClientManager) *ServiceFactory {
	return &ServiceFactory{
		managementClient: managementClient,
		logger:           logrus.WithField("component", "PricingServiceFactory"),
	}
}

// CreateCommonPricingService 创建通用核价服务
func (f *ServiceFactory) CreateCommonPricingService() service.CommonPricingService {
	// 创建成本价提供者
	costProvider := service.NewDefaultCostPriceProvider(f.managementClient)

	// 创建价格计算器
	calculator := service.NewDefaultPricingCalculator()

	// 创建决策制定者
	decisionMaker := service.NewDefaultPricingDecisionMaker(calculator, costProvider)

	// 创建通用核价服务
	pricingService := service.NewDefaultCommonPricingService(decisionMaker)

	// 注册平台适配器
	f.registerAdapters(pricingService, costProvider)

	f.logger.Info("通用核价服务创建完成")
	return pricingService
}

// registerAdapters 注册平台适配器
func (f *ServiceFactory) registerAdapters(pricingService service.CommonPricingService, costProvider service.CostPriceProvider) {
	// 注册TEMU适配器
	temuAdapter := adapter.NewTemuAdapter(costProvider)
	pricingService.RegisterAdapter(temuAdapter.GetPlatformName(), temuAdapter)

	// 注册SHEIN适配器
	sheinAdapter := adapter.NewSheinAdapter(costProvider)
	pricingService.RegisterAdapter(sheinAdapter.GetPlatformName(), sheinAdapter)

	f.logger.Info("平台适配器注册完成: TEMU, SHEIN")
}

// CreateTemuAdapter 创建TEMU适配器
func (f *ServiceFactory) CreateTemuAdapter() *adapter.TemuAdapter {
	costProvider := service.NewDefaultCostPriceProvider(f.managementClient)
	return adapter.NewTemuAdapter(costProvider)
}

// CreateSheinAdapter 创建SHEIN适配器
func (f *ServiceFactory) CreateSheinAdapter() *adapter.SheinAdapter {
	costProvider := service.NewDefaultCostPriceProvider(f.managementClient)
	return adapter.NewSheinAdapter(costProvider)
}

// CreatePricingCalculator 创建价格计算器
func (f *ServiceFactory) CreatePricingCalculator() service.PricingCalculator {
	return service.NewDefaultPricingCalculator()
}

// CreateCostPriceProvider 创建成本价提供者
func (f *ServiceFactory) CreateCostPriceProvider() service.CostPriceProvider {
	return service.NewDefaultCostPriceProvider(f.managementClient)
}

// CreateDecisionMaker 创建决策制定者
func (f *ServiceFactory) CreateDecisionMaker() service.PricingDecisionMaker {
	costProvider := f.CreateCostPriceProvider()
	calculator := f.CreatePricingCalculator()
	return service.NewDefaultPricingDecisionMaker(calculator, costProvider)
}
