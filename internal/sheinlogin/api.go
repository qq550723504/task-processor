package sheinlogin

import (
	"net/http"
	"strconv"
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

func (h *Handler) ListAccounts(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListAccounts(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *Handler) Login(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	var req LoginRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	result, err := h.svc.Login(c.Request.Context(), tenantID, storeID, req)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	statusCode := http.StatusOK
	if result.WaitingForVerifyCode {
		statusCode = http.StatusAccepted
	}
	c.JSON(statusCode, result)
}

func (h *Handler) Status(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	status, err := h.svc.Status(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func (h *Handler) SubmitVerifyCode(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	var req VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "verify code is required"})
		return
	}
	if err := h.svc.SubmitVerifyCode(c.Request.Context(), tenantID, storeID, req.Code, req.ExpireSeconds); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) CancelVerifyCodeWait(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	cancelled, err := h.svc.CancelVerifyCodeWait(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "cancelled": cancelled})
}

func (h *Handler) ClearCookie(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	if err := h.svc.ClearCookie(c.Request.Context(), tenantID, storeID); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ClearLastFailure(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	if err := h.svc.ClearLastFailure(c.Request.Context(), tenantID, storeID); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) GetLastFailure(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	detail, err := h.svc.GetLastFailureDetail(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

func (h *Handler) AdminPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, adminPageHTML)
}

func parseStoreID(c *gin.Context) (int64, bool) {
	value := c.Param("store_id")
	storeID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || storeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid store_id"})
		return 0, false
	}
	return storeID, true
}

func requireTenantID(c *gin.Context) (int64, bool) {
	tenantID, err := requestTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return 0, false
	}
	return tenantID, true
}

func statusCodeForTenantScopedError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "tenant id is required") {
		return http.StatusUnauthorized
	}
	if strings.Contains(message, "account not found for tenant") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

const adminPageHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>SHEIN Login</title>
<style>
body{font-family:Arial,sans-serif;margin:24px}
table{border-collapse:collapse;width:100%}
th,td{padding:8px;border:1px solid #ddd;vertical-align:top}
button{margin-right:8px}
.muted{color:#666;font-size:12px}
.error{color:#b42318}
code{font-family:Consolas,monospace;font-size:12px;word-break:break-all}
.actions{display:flex;flex-wrap:wrap;gap:8px}
.actions button{margin-right:0}
.btn-primary{background:#111827;color:#fff;border:1px solid #111827}
.btn-warning{background:#b42318;color:#fff;border:1px solid #b42318}
.btn-muted{opacity:.7}
</style>
</head><body>
<h1>SHEIN Login</h1>
<p><button onclick="load()">刷新</button></p>
<table><thead><tr><th>Store</th><th>Username</th><th>Cookie TTL</th><th>Verify</th><th>Last Failure</th><th>Recommended</th><th>Actions</th></tr></thead><tbody id="rows"></tbody></table>
<script>
function esc(value){
  return String(value ?? '').replace(/[&<>"']/g, (ch) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
}
function renderFailure(item){
  const failure=item.last_failure;
  if(!failure){ return '<span class="muted">-</span>'; }
  const parts=[];
  if(failure.error_code){ parts.push('<div class="error"><strong>'+esc(failure.error_code)+'</strong></div>'); }
  if(failure.page_state){ parts.push('<div class="muted">state: '+esc(failure.page_state)+'</div>'); }
  if(failure.action_message){ parts.push('<div>'+esc(failure.action_message)+'</div>'); }
  if(failure.error_message){ parts.push('<div>'+esc(failure.error_message)+'</div>'); }
  if(failure.captured_at){ parts.push('<div class="muted">'+esc(failure.captured_at)+'</div>'); }
  if(failure.artifact_path){ parts.push('<div><code>'+esc(failure.artifact_path)+'</code></div>'); }
  return parts.join('');
}
function actionButtonClass(item, actionName){
  const actionKey=(item.recommended_action&&item.recommended_action.key)||'';
  const waiting=!!item.waiting_for_verify_code;
  if(actionName==='code' && (waiting || actionKey==='submit_verify_code')){
    return 'btn-primary';
  }
  if(actionName==='login' && (actionKey==='retry_login' || actionKey==='check_cookie_persistence')){
    return 'btn-primary';
  }
  if(actionName==='clearCookie' && actionKey==='check_credentials'){
    return 'btn-warning';
  }
  if(actionName==='showFailure' && actionKey==='inspect_artifact'){
    return 'btn-primary';
  }
  if(actionName==='showFailure' && (actionKey==='use_master_account' || actionKey==='fix_account_permission')){
    return 'btn-primary';
  }
  if(actionName==='login' && (actionKey==='use_master_account' || actionKey==='fix_account_permission')){
    return 'btn-muted';
  }
  if(actionName==='code' && !waiting && actionKey!=='submit_verify_code'){
    return 'btn-muted';
  }
  return '';
}
function renderActions(item){
  const id=item.account.store_id;
  return '<div class="actions">'
    +'<button class="'+actionButtonClass(item,'login')+'" onclick="login('+id+')">登录</button>'
    +'<button class="'+actionButtonClass(item,'code')+'" onclick="code('+id+')">验证码</button>'
    +'<button class="'+actionButtonClass(item,'clearCookie')+'" onclick="clearCookie('+id+')">清Cookie</button>'
    +'<button class="'+actionButtonClass(item,'showFailure')+'" onclick="showFailure('+id+')">详情</button>'
    +'<button onclick="clearFailure('+id+')">清失败</button>'
    +'</div>';
}
function renderRecommendedAction(item){
  const action=item.recommended_action||{};
  if(!action.key && !action.message){ return '<span class="muted">-</span>'; }
  const parts=[];
  if(action.key){ parts.push('<div><strong>'+esc(action.key)+'</strong></div>'); }
  if(action.message){ parts.push('<div class="muted">'+esc(action.message)+'</div>'); }
  return parts.join('');
}
async function load(){
  const res=await fetch('/api/v1/shein-login/accounts'); const data=await res.json(); const rows=document.getElementById('rows'); rows.innerHTML='';
  for(const item of (data.data||[])){ const tr=document.createElement('tr'); tr.innerHTML=
  '<td>'+item.account.store_id+'</td><td>'+item.account.username+'</td><td>'+(item.cookie_ttl||0)+'</td><td>'+(item.waiting_for_verify_code?'waiting':'-')+'</td><td>'+renderFailure(item)+'</td>'+
  '<td>'+renderRecommendedAction(item)+'</td>'+
  '<td>'+renderActions(item)+'</td>'; rows.appendChild(tr);}
}
async function login(id){await fetch('/api/v1/shein-login/accounts/'+id+'/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({force_login:true})}); load();}
async function code(id){const v=prompt('验证码'); if(!v)return; await fetch('/api/v1/shein-login/accounts/'+id+'/verify-code',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code:v})}); load();}
async function clearCookie(id){await fetch('/api/v1/shein-login/accounts/'+id+'/cookie',{method:'DELETE'}); load();}
async function showFailure(id){
  const res=await fetch('/api/v1/shein-login/accounts/'+id+'/last-failure'); const data=await res.json();
  if(!data.data){ alert('没有最近失败记录'); return; }
  const item=data.data;
  alert(
    '错误码: '+(item.error_code||'-')+'\n'
    +'页面状态: '+(item.page_state||'-')+'\n'
    +'建议动作: '+(item.action_key||'-')+'\n'
    +'建议说明: '+(item.action_message||'-')+'\n'
    +'时间: '+(item.captured_at||'-')+'\n'
    +'阶段: '+(item.stage||'-')+'\n'
    +'URL: '+(item.url||'-')+'\n'
    +'标题: '+(item.title||'-')+'\n'
    +'仍在登录页: '+(item.on_login_page?'yes':'no')+'\n'
    +'请求失败弹层: '+(item.request_failure_modal?'yes':'no')+'\n'
    +'登录表单可见: '+(item.login_form_visible?'yes':'no')+'\n'
    +'卖家中心可见: '+(item.seller_hub_visible?'yes':'no')+'\n'
    +'验证码区域可见: '+(item.verification_visible?'yes':'no')+'\n'
    +'权限提示可见: '+(item.permission_visible?'yes':'no')+'\n'
    +'协议提示可见: '+(item.agreement_visible?'yes':'no')+'\n'
    +'账号密码错误可见: '+(item.credential_error_visible?'yes':'no')+'\n'
    +'页面错误: '+(item.login_error||'-')+'\n'
    +'Artifact: '+(item.artifact_path||'-')+'\n\n'
    +'Selector States: '+JSON.stringify(item.selector_states||{})+'\n\n'
    +(item.body_text||'')
  );
}
async function clearFailure(id){await fetch('/api/v1/shein-login/accounts/'+id+'/last-failure',{method:'DELETE'}); load();}
load();
</script></body></html>`
