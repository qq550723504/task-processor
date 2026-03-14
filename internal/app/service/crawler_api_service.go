// Package service 提供应用服务编排
package service

import (
	"context"
	"fmt"
	"net/http"

	crawlerapp "task-processor/internal/app/crawler/amazon"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/http/handler"

	"github.com/sirupsen/logrus"
)

// CrawlerAPIService Amazon 爬虫 API 服务
type CrawlerAPIService struct {
	config         *config.Config
	logger         *logrus.Logger
	crawlerService *crawlerapp.Service
	httpServer     *http.Server
	port           int
}

// NewCrawlerAPIService 创建 API 服务
func NewCrawlerAPIService(cfg *config.Config, logger *logrus.Logger, port int) *CrawlerAPIService {
	return &CrawlerAPIService{
		config:         cfg,
		logger:         logger,
		crawlerService: crawlerapp.NewService(cfg, logger),
		port:           port,
	}
}

// Start 启动服务
func (s *CrawlerAPIService) Start(ctx context.Context) error {
	// 启动爬虫应用服务
	if err := s.crawlerService.Start(ctx); err != nil {
		return fmt.Errorf("启动爬虫服务失败: %w", err)
	}

	// 创建 HTTP 处理器
	httpHandler := handler.NewCrawlerHandler(s.crawlerService, s.logger)

	// 注册路由
	mux := httpHandler.RegisterRoutes()

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// 启动 HTTP 服务器
	go func() {
		s.logger.Infof("🚀 Amazon 爬虫 API 服务启动在端口: %d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务
func (s *CrawlerAPIService) Stop(ctx context.Context) error {
	s.logger.Info("正在停止 Amazon 爬虫 API 服务...")

	// 停止 HTTP 服务器
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Errorf("停止 HTTP 服务器失败: %v", err)
		}
	}

	// 停止爬虫服务
	if err := s.crawlerService.Stop(ctx); err != nil {
		s.logger.Errorf("停止爬虫服务失败: %v", err)
	}

	s.logger.Info("✅ Amazon 爬虫 API 服务已停止")
	return nil
}
