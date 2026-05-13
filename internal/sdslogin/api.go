package sdslogin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Health(c.Request.Context()))
}

func (h *Handler) Status(c *gin.Context) {
	status, err := h.svc.Status(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	payload, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) ManualLogin(c *gin.Context) {
	var req ManualLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.MerchantName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "merchant_name, username and password are required"})
		return
	}
	payload, err := h.svc.ManualLogin(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) GetAuthState(c *gin.Context) {
	payload, err := h.svc.loadPayload()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

func (h *Handler) ClearState(c *gin.Context) {
	if err := h.svc.ClearState(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) AdminPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, adminPageHTML)
}

const adminPageHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>SDS Login</title>
<style>
body{font-family:Arial,sans-serif;margin:24px}
table{border-collapse:collapse;width:100%}
th,td{padding:8px;border:1px solid #ddd;vertical-align:top}
button{margin-right:8px}
.muted{color:#666;font-size:12px}
code{font-family:Consolas,monospace;font-size:12px;word-break:break-all}
.grid{display:grid;grid-template-columns:repeat(2,minmax(240px,1fr));gap:12px;max-width:960px;margin:16px 0}
.grid input{padding:8px}
</style>
</head><body>
<h1>SDS Login</h1>
<p><button onclick="load()">刷新</button><button onclick="login(true)">强制登录</button><button onclick="clearState()">清状态</button></p>
<div class="grid">
  <input id="tenantId" placeholder="tenant_id">
  <input id="identifier" placeholder="identifier">
  <input id="merchantName" placeholder="merchant_name">
  <input id="username" placeholder="username">
  <input id="password" type="password" placeholder="password">
  <div><button onclick="manualLogin()">手动登录</button></div>
</div>
<div id="status">加载中...</div>
<script>
function esc(value){return String(value ?? '').replace(/[&<>"']/g, (ch) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));}
async function load(){
  const [statusRes, authRes]=await Promise.all([fetch('/api/v1/sds-login/status'), fetch('/api/v1/sds-login/auth-state')]);
  const statusData=await statusRes.json(); const authData=await authRes.json();
  const status=statusData.data||{}; const auth=authData.data||{};
  if(!document.getElementById('tenantId').value && status.tenant_id){ document.getElementById('tenantId').value=status.tenant_id; }
  if(!document.getElementById('identifier').value && status.identifier){ document.getElementById('identifier').value=status.identifier; }
  if(!document.getElementById('merchantName').value && status.merchant_name){ document.getElementById('merchantName').value=status.merchant_name; }
  if(!document.getElementById('username').value && status.username){ document.getElementById('username').value=status.username; }
  document.getElementById('status').innerHTML=
    '<table><tbody>'
    +'<tr><th>Tenant</th><td>'+esc(status.tenant_id||'-')+'</td></tr>'
    +'<tr><th>Identifier</th><td>'+esc(status.identifier||'-')+'</td></tr>'
    +'<tr><th>Merchant</th><td>'+esc(status.merchant_name||'-')+'</td></tr>'
    +'<tr><th>Username</th><td>'+esc(status.username||'-')+'</td></tr>'
    +'<tr><th>Has Cookie</th><td>'+String(!!status.has_cookie)+'</td></tr>'
    +'<tr><th>Has Access Token</th><td>'+String(!!status.has_access_token)+'</td></tr>'
    +'<tr><th>Waiting</th><td>'+String(!!status.waiting_for_verify_code)+'</td></tr>'
    +'<tr><th>In Progress</th><td>'+String(!!status.login_in_progress)+'</td></tr>'
    +'<tr><th>Last Error</th><td>'+esc(status.last_error||'-')+'</td></tr>'
    +'<tr><th>Issued At</th><td>'+esc(status.issued_at||'-')+'</td></tr>'
    +'<tr><th>Source</th><td>'+esc(status.source||'-')+'</td></tr>'
    +'<tr><th>Current URL</th><td>'+esc(auth.current_url||'-')+'</td></tr>'
    +'<tr><th>Token</th><td><code>'+esc(auth.access_token||'-')+'</code></td></tr>'
    +'</tbody></table>';
}
async function login(force){
  const res=await fetch('/api/v1/sds-login/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({force_login:!!force,headless:false})});
  const data=await res.json();
  alert((data.success?'成功: ':'结果: ')+(data.message||''));
  load();
}
async function manualLogin(){
  const payload={
    tenant_id: document.getElementById('tenantId').value.trim(),
    identifier: document.getElementById('identifier').value.trim(),
    merchant_name: document.getElementById('merchantName').value.trim(),
    username: document.getElementById('username').value.trim(),
    password: document.getElementById('password').value,
    force_login: true,
    headless: false
  };
  const res=await fetch('/api/v1/sds-login/manual-login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
  const data=await res.json();
  alert((data.success?'成功: ':'结果: ')+(data.message||''));
  load();
}
async function clearState(){ await fetch('/api/v1/sds-login/state',{method:'DELETE'}); load(); }
load();
</script></body></html>`
