// Package bootstrap 提供简化的服务注册表实现
package bootstrap

import (
	"fmt"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/di"

	"github.com/sirupsen/logrus"
)

// ServiceRegistrySimple 简化的服务注册表
type ServiceRegistrySimple struct {
	logger *logrus.Logger
}

// NewServiceRegistrySimple 创建简化的服务注册表
func NewServiceRegistrySimple(logger *logrus.Logger) *ServiceRegistrySimple {
	return &ServiceRegistrySimple{
		logger: logger,
	}
}

// RegisterAllServices 注册所有业务服务到容器
func (s *ServiceRegistrySimple) RegisterAllServices(container di.Container, cfg *config.Config) error {
	// 注册基础服务
	s.registerBaseServices(container)

	// 注册认证服务
	if err := s.registerAuthServices(container); err != nil {
		return fmt.Errorf("注册认证服务失败: %w", err)
	}

	// 注册共享资源
	if err := s.registerSharedResources(container); err != nil {
		return fmt.Errorf("注册共享资源失败: %w", err)
	}

	// 注册应用服务
	if err := s.registerApplicationServices(container); err != nil {
		return fmt.Errorf("注册应用服务失败: %w", err)
	}

	return nil
}

// registerBaseServices 注册基础服务
func (s *ServiceRegistrySimple) registerBaseServices(_ di.Container) {
	s.logger.Debug("注册基础服务...")
}

// registerAuthServices 注册认证服务
func (s *ServiceRegistrySimple) registerAuthServices(container di.Container) error {
	s.logger.Debug("注册认证服务...")

	// 直接注册认证客户端，无需中间层
	if err := container.RegisterSingleton("authClient", func(c di.Container) (any, error) {
		loggerInstance, err := c.Get("logger")
		if err != nil {
			return nil, fmt.Errorf("获取日志器失败: %w", err)
		}
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}

		logger := loggerInstance.(*logrus.Logger)
		cfg := configInstance.(*config.Config)

		tenantID := cfg.Management.TenantID
		if tenantID == "" {
			tenantID = "1"
		}

		s.logger.Info("初始化客户端凭证授权...")
		client := auth.NewClientCredentialsAuthClient(
			cfg.Management.BaseURL,
			cfg.Management.ClientID,
			cfg.Management.ClientSecret,
			tenantID,
			logger,
		)

		if _, err := client.GetAccessToken(); err != nil {
			return nil, fmt.Errorf("获取访问令牌失败: %w", err)
		}

		return client, nil
	}); err != nil {
		return err
	}

	return nil
}

// registerSharedResources 注册共享资源
func (s *ServiceRegistrySimple) registerSharedResources(container di.Container) error {
	s.logger.Debug("注册共享资源...")

	// 注册Amazon处理器
	if err := container.RegisterSingleton("amazonProcessor", func(c di.Container) (any, error) {
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}
		config := configInstance.(*config.Config)
		return amazon.NewAmazonProcessor(config), nil
	}); err != nil {
		return err
	}

	// 注册管理客户端
	if err := container.RegisterSingleton("managementClient", func(c di.Container) (any, error) {
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}
		authClientInstance, err := c.Get("authClient")
		if err != nil {
			return nil, fmt.Errorf("获取认证客户端失败: %w", err)
		}

		config := configInstance.(*config.Config)
		authClient := authClientInstance.(*auth.ClientCredentialsAuthClient)

		// 获取访问令牌
		accessToken, err := authClient.GetAccessToken()
		if err != nil {
			return nil, fmt.Errorf("获取访问令牌失败: %w", err)
		}

		// 创建管理客户端
		managementClient := management.NewClientManager(&config.Management)

		// 设置访问令牌
		client := managementClient.GetClient()
		client.SetUserToken(accessToken, config.Management.TenantID)

		return managementClient, nil
	}); err != nil {
		return err
	}

	return nil
}

// registerApplicationServices 注册应用服务
func (s *ServiceRegistrySimple) registerApplicationServices(container di.Container) error {
	s.logger.Debug("注册应用服务...")

	// 注册处理器服务
	if err := container.RegisterSingleton("processorService", func(c di.Container) (any, error) {
		loggerInstance, err := c.Get("logger")
		if err != nil {
			return nil, fmt.Errorf("获取日志器失败: %w", err)
		}
		managementClientInstance, err := c.Get("managementClient")
		if err != nil {
			return nil, fmt.Errorf("获取管理客户端失败: %w", err)
		}
		amazonProcessorInstance, err := c.Get("amazonProcessor")
		if err != nil {
			return nil, fmt.Errorf("获取Amazon处理器失败: %w", err)
		}

		logger := loggerInstance.(*logrus.Logger)
		managementClient := managementClientInstance.(*management.ClientManager)
		amazonProcessor := amazonProcessorInstance.(*amazon.AmazonProcessor)

		return runner.NewProcessorServiceWithDependencies(logger, managementClient, amazonProcessor), nil
	}); err != nil {
		return err
	}

	// 注册平台处理器
	platformRegistry := NewPlatformProcessorRegistry(s.logger)
	if err := platformRegistry.RegisterPlatformProcessors(container); err != nil {
		return fmt.Errorf("注册平台处理器失败: %w", err)
	}

	// 注册调度服务
	if err := container.RegisterSingleton("schedulerService", func(c di.Container) (any, error) {
		loggerInstance, err := c.Get("logger")
		if err != nil {
			return nil, fmt.Errorf("获取日志器失败: %w", err)
		}
		managementClientInstance, err := c.Get("managementClient")
		if err != nil {
			return nil, fmt.Errorf("获取管理客户端失败: %w", err)
		}
		configInstance, err := c.Get("config")
		if err != nil {
			return nil, fmt.Errorf("获取配置失败: %w", err)
		}
		amazonProcessorInstance, err := c.Get("amazonProcessor")
		if err != nil {
			return nil, fmt.Errorf("获取Amazon处理器失败: %w", err)
		}

		logger := loggerInstance.(*logrus.Logger)
		managementClient := managementClientInstance.(*management.ClientManager)
		config := configInstance.(*config.Config)
		amazonProcessor := amazonProcessorInstance.(*amazon.AmazonProcessor)

		return runner.NewSchedulerServiceWithAmazon(logger, managementClient, config, amazonProcessor), nil
	}); err != nil {
		return err
	}

	return nil
}
