// Package service 提供应用服务编排
package service

import (
	"context"
	"fmt"
	"net/http"

	"task-processor/internal/application/crawler1688"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/http/handler"

	"github.com/sirupsen/logrus"
)

// Crawler1688APIService 1688爬虫 API 服务
type Crawler1688APIService struct {
	config         *config.Config
	logger         *logrus.Logger
	crawlerService *crawler1688.Service
	httpServer     *http.Server
	port           int
}

// New1688CrawlerAPIService 创建 1688 API 服务
func New1688CrawlerAPIService(cfg *config.Config, logger *logrus.Logger, port int) *Crawler1688APIService {
	return &Crawler1688APIService{
		config:         cfg,
		logger:         logger,
		crawlerService: crawler1688.NewService(cfg, logger),
		port:           port,
	}
}

// Start 启动服务
func (s *Crawler1688APIService) Start(ctx context.Context) error {
	// 启动爬虫应用服务
	if err := s.crawlerService.Start(ctx); err != nil {
		return fmt.Errorf("启动1688爬虫服务失败: %w", err)
	}

	// 创建 HTTP 处理器
	httpHandler := handler.NewCrawler1688Handler(s.crawlerService, s.logger)

	// 注册路由
	mux := httpHandler.RegisterRoutes()

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// 启动 HTTP 服务器
	go func() {
		s.logger.Infof("🚀 1688 爬虫 API 服务启动在端口: %d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务
func (s *Crawler1688APIService) Stop(ctx context.Context) error {
	s.logger.Info("正在停止 1688 爬虫 API 服务...")

	// 停止 HTTP 服务器
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Errorf("停止 HTTP 服务器失败: %v", err)
		}
	}

	// 停止爬虫服务
	if err := s.crawlerService.Stop(ctx); err != nil {
		s.logger.Errorf("停止1688爬虫服务失败: %v", err)
	}

	s.logger.Info("✅ 1688 爬虫 API 服务已停止")
	return nil
}
