// Package amazon 提供 Amazon 爬虫 API 服务
package amazon

import (
	"context"
	"fmt"
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/httpx"

	"github.com/sirupsen/logrus"
)

// APIService Amazon 爬虫 HTTP API 服务
type APIService struct {
	config         *config.Config
	logger         *logrus.Logger
	crawlerService *Service
	httpServer     *http.Server
	port           int
}

// NewAPIService 创建 API 服务
func NewAPIService(cfg *config.Config, logger *logrus.Logger, port int) *APIService {
	return &APIService{
		config:         cfg,
		logger:         logger,
		crawlerService: NewService(cfg, logger),
		port:           port,
	}
}

// Start 启动服务
func (s *APIService) Start(ctx context.Context) error {
	if err := s.crawlerService.Start(ctx); err != nil {
		return fmt.Errorf("启动爬虫服务失败: %w", err)
	}

	httpHandler := httpx.NewCrawlerHandler(s.crawlerService, s.logger)
	mux := httpHandler.RegisterRoutes()

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		s.logger.Infof("🚀 Amazon 爬虫 API 服务启动在端口: %d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务
func (s *APIService) Stop(ctx context.Context) error {
	s.logger.Info("正在停止 Amazon 爬虫 API 服务...")

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Errorf("停止 HTTP 服务器失败: %v", err)
		}
	}

	if err := s.crawlerService.Stop(ctx); err != nil {
		s.logger.Errorf("停止爬虫服务失败: %v", err)
	}

	s.logger.Info("✅ Amazon 爬虫 API 服务已停止")
	return nil
}
