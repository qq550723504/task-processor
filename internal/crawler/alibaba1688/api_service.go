// Package alibaba1688 提供 1688 爬虫 API 服务
package alibaba1688

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/listingadmin"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

var buildListingAdminStoreRepository = listingkithttpapi.BuildListingAdminStoreRepository

// APIService 1688 爬虫 HTTP API 服务
type APIService struct {
	config            *config.Config
	logger            *logrus.Logger
	crawlerService    *Service
	httpServer        *http.Server
	port              int
	repositoryClosers []func() error
}

// NewAPIService 创建 1688 API 服务
func NewAPIService(cfg *config.Config, logger *logrus.Logger, port int) *APIService {
	if !hasConfiguredDatabase(cfg) {
		return newAPIService(cfg, logger, port, nil, nil)
	}
	repository, closers, err := buildListingAdminStoreRepository(cfg, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("1688 account profile repository unavailable")
		}
		return newAPIService(cfg, logger, port, nil, nil)
	}
	if repository == nil {
		return newAPIService(cfg, logger, port, nil, closers)
	}
	return newAPIService(cfg, logger, port, NewAccountProfileResolver(repository, cfg.Platforms.Alibaba1688.ProfileRootDir), closers)
}

// NewAPIServiceWithStoreRepository creates an API service that can resolve tenant-owned 1688 account profiles.
func NewAPIServiceWithStoreRepository(cfg *config.Config, logger *logrus.Logger, port int, repository listingadmin.StoreRepository) *APIService {
	resolver := NewAccountProfileResolver(repository, cfg.Platforms.Alibaba1688.ProfileRootDir)
	return newAPIService(cfg, logger, port, resolver, nil)
}

func newAPIService(cfg *config.Config, logger *logrus.Logger, port int, resolver AccountProfileResolver, repositoryClosers []func() error) *APIService {
	return &APIService{
		config:            cfg,
		logger:            logger,
		crawlerService:    NewService(cfg, logger, resolver),
		port:              port,
		repositoryClosers: append([]func() error(nil), repositoryClosers...),
	}
}

// Start 启动服务
func (s *APIService) Start(ctx context.Context) error {
	if err := s.crawlerService.Start(ctx); err != nil {
		return fmt.Errorf("启动1688爬虫服务失败: %w", err)
	}

	if s.config != nil {
		listingkithttpapi.ConfigureListingKitZitadelAuth(s.config.ListingKit.Zitadel)
	}
	httpHandler := httpx.NewCrawler1688Handler(s.crawlerService, s.logger)
	mux := httpHandler.RegisterRoutes()
	handler := wrapZitadelAuthMiddleware(mux, listingkithttpapi.NewZitadelAuthMiddlewareFromEnv())

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: handler,
	}

	go func() {
		s.logger.Infof("🚀 1688 爬虫 API 服务启动在端口: %d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("HTTP 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务
func (s *APIService) Stop(ctx context.Context) error {
	s.logger.Info("正在停止 1688 爬虫 API 服务...")

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Errorf("停止 HTTP 服务器失败: %v", err)
		}
	}

	if err := s.crawlerService.Stop(ctx); err != nil {
		s.logger.Errorf("停止1688爬虫服务失败: %v", err)
	}
	for _, closeRepository := range s.repositoryClosers {
		if closeRepository != nil {
			if err := closeRepository(); err != nil && s.logger != nil {
				s.logger.Warn("关闭1688账号配置仓储失败")
			}
		}
	}
	s.repositoryClosers = nil

	s.logger.Info("✅ 1688 爬虫 API 服务已停止")
	return nil
}

func hasConfiguredDatabase(cfg *config.Config) bool {
	return cfg != nil && cfg.Database != nil && strings.TrimSpace(cfg.Database.Host) != ""
}

func wrapZitadelAuthMiddleware(next http.Handler, middleware gin.HandlerFunc) http.Handler {
	if middleware == nil {
		return next
	}
	router := gin.New()
	router.Use(middleware)
	router.Any("/*path", func(c *gin.Context) {
		next.ServeHTTP(c.Writer, c.Request)
	})
	return router
}
