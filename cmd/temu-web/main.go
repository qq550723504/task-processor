package main

import (
	"embed"
	"html/template"
	"net/http"

	"task-processor/cmd/temu-web/handlers"
	"task-processor/cmd/temu-web/middleware"
	"task-processor/cmd/temu-web/server"
	"task-processor/cmd/temu-web/utils"
	"task-processor/common/config"

	"github.com/sirupsen/logrus"
)

//go:embed templates/*.html
var templates embed.FS

const appVersion = "1.0.0"

func main() {
	// Setup logger
	logger := utils.SetupLogger()

	// Ensure working directory is correct
	if err := utils.EnsureWorkingDirectory(logger); err != nil {
		logger.Fatalf("设置工作目录失败: %v", err)
	}

	// Load configuration
	cfg := config.LoadConfig("temu")
	if cfg == nil {
		logger.Fatal("配置加载失败")
	}
	logger.Infof("配置加载成功，服务器端口: %d", cfg.Server.Port)

	// Log environment info
	utils.LogEnvironmentInfo(logger, appVersion)

	// Create server instance
	srv := server.New(cfg, templates, logger)

	// Initialize server components
	if err := srv.Initialize(); err != nil {
		logger.Fatalf("服务器初始化失败: %v", err)
	}

	// Setup routes
	setupRoutes(srv, logger)

	// Start server
	if err := srv.Start(); err != nil {
		logger.Fatalf("启动服务器失败: %v", err)
	}
}

func setupRoutes(srv *server.Server, logger *logrus.Logger) {
	// Parse templates
	loginTmpl, err := template.ParseFS(templates, "templates/login.html")
	if err != nil {
		logger.Fatalf("解析登录模板失败: %v", err)
	}

	dashboardTmpl, err := template.ParseFS(templates, "templates/dashboard.html")
	if err != nil {
		logger.Fatalf("解析仪表板模板失败: %v", err)
	}

	// Create handlers
	loginHandler := handlers.New(srv.GetSessionManager(), srv.GetPasswordClient(), loginTmpl, logger, srv)
	dashboardHandler := handlers.New(srv.GetSessionManager(), srv.GetPasswordClient(), dashboardTmpl, logger, srv)

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(srv.GetSessionManager(), logger)

	// Setup routes
	http.HandleFunc("/", loginHandler.LoginPageHandler)
	http.HandleFunc("/api/login", loginHandler.LoginHandler)
	http.HandleFunc("/api/logout", loginHandler.LogoutHandler)
	http.HandleFunc("/api/start-processor", loginHandler.StartProcessorHandler)
	http.HandleFunc("/api/stop-processor", loginHandler.StopProcessorHandler)
	http.HandleFunc("/api/processor-status", loginHandler.ProcessorStatusHandler)
	http.HandleFunc("/dashboard", authMiddleware.RequireAuth(dashboardHandler.DashboardHandler))
}
