package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"task-processor/common/auth"

	"github.com/sirupsen/logrus"
)

// Handler holds the dependencies for HTTP handlers
type Handler struct {
	sessionManager   *auth.SessionManager
	passwordClient   *auth.PasswordAuthClient
	templates        *template.Template
	logger           *logrus.Logger
	processorManager ProcessorManager
}

// ProcessorManager interface for task processor operations
type ProcessorManager interface {
	IsRunning() bool
	StartProcessor() error
	StopProcessor() error
	InitializeProcessor()
	SetUserToken(accessToken, tenantID string)
}

// New creates a new handler instance
func New(sessionManager *auth.SessionManager, passwordClient *auth.PasswordAuthClient,
	templates *template.Template, logger *logrus.Logger, processorManager ProcessorManager) *Handler {
	return &Handler{
		sessionManager:   sessionManager,
		passwordClient:   passwordClient,
		templates:        templates,
		logger:           logger,
		processorManager: processorManager,
	}
}

// LoginPageHandler handles the login page
func (h *Handler) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is already logged in
	cookie, err := r.Cookie("session_token")
	if err == nil {
		_, exists := h.sessionManager.GetSession(cookie.Value)
		if exists {
			h.logger.Infof("用户已登录，重定向到dashboard: %s", cookie.Value)
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
	}

	data := map[string]any{
		"TenantID": "",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.Execute(w, data); err != nil {
		h.logger.Errorf("模板执行失败: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// LoginHandler handles login requests
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"success": false,
			"message": "Method not allowed",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TenantID string `json:"tenant_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "无效的请求格式",
		})
		return
	}

	if req.Username == "" || req.Password == "" {
		h.respondJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "请填写用户名和密码",
		})
		return
	}

	if req.TenantID == "" {
		req.TenantID = "default"
	}

	session, err := h.passwordClient.Login(req.Username, req.Password, req.TenantID)
	if err != nil {
		h.logger.Errorf("登录失败: %v", err)
		h.respondJSON(w, http.StatusUnauthorized, map[string]any{
			"success": false,
			"message": fmt.Sprintf("登录失败: %v", err),
		})
		return
	}

	h.logger.Infof("用户登录成功: %s (租户: %s)", session.Username, session.TenantID)

	h.sessionManager.SaveSession(session.Username, session)

	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    session.Username,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   30 * 24 * 3600, // 30天
	}
	http.SetCookie(w, cookie)

	h.processorManager.InitializeProcessor()
	h.processorManager.SetUserToken(session.AccessToken, session.TenantID)
	go h.processorManager.StartProcessor()

	h.respondJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "登录成功",
		"username": session.Username,
	})
}

// LogoutHandler handles logout requests
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		h.sessionManager.RemoveSession(cookie.Value)
	}

	h.processorManager.StopProcessor()

	http.SetCookie(w, &http.Cookie{
		Name:   "session_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	h.respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "登出成功",
	})
}

// StartProcessorHandler handles processor start requests
func (h *Handler) StartProcessorHandler(w http.ResponseWriter, r *http.Request) {
	if h.processorManager.IsRunning() {
		h.respondJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "任务处理器已在运行",
		})
		return
	}

	go h.processorManager.StartProcessor()

	h.respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "任务处理器启动中",
	})
}

// StopProcessorHandler handles processor stop requests
func (h *Handler) StopProcessorHandler(w http.ResponseWriter, r *http.Request) {
	go h.processorManager.StopProcessor()
	h.respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "任务处理器停止中",
	})
}

// ProcessorStatusHandler handles processor status requests
func (h *Handler) ProcessorStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := "stopped"
	running := h.processorManager.IsRunning()
	if running {
		status = "running"
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"running": running,
	})
}

// DashboardHandler handles dashboard page requests
func (h *Handler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Dashboard访问请求")

	cookie, err := r.Cookie("session_token")
	if err != nil {
		h.logger.Errorf("获取session_token cookie失败: %v", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	session, exists := h.sessionManager.GetSession(cookie.Value)
	if !exists {
		h.logger.Errorf("会话不存在: %s", cookie.Value)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	h.logger.Infof("会话验证成功: %s", session.Username)

	data := map[string]any{
		"Username": session.Username,
		"TenantID": session.TenantID,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.Execute(w, data); err != nil {
		h.logger.Errorf("模板执行失败: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
