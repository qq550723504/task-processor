# 函数清单 - Infra/Auth 模块

生成时间: 2026-03-10

## infra/auth 模块

### client_credentials.go
```go
func NewClientCredentialsAuthClient(baseURL, clientID, clientSecret, tenantID string, logger *logrus.Logger) *ClientCredentialsAuthClient
func (c *ClientCredentialsAuthClient) GetAccessToken() (string, error)
func (c *ClientCredentialsAuthClient) GetTenantID() string
func (c *ClientCredentialsAuthClient) isTokenValid() bool
func (c *ClientCredentialsAuthClient) RefreshToken() error
```

### token_fetcher.go
```go
func (c *ClientCredentialsAuthClient) fetchAccessToken() (string, error)
func (c *ClientCredentialsAuthClient) buildTokenRequest(tokenURL string) (*http.Request, error)
func (c *ClientCredentialsAuthClient) parseAndSaveToken(body []byte) (string, error)
func (c *ClientCredentialsAuthClient) parseTokenResponse(body []byte) (string, int64, error)
func (c *ClientCredentialsAuthClient) calculateExpiresAt(expiresIn int64) time.Time
```

### manager.go
```go
func NewSessionManager(tokenStore TokenStore, logger *logrus.Logger) *SessionManager
func (sm *SessionManager) CreateSession(username, tenantID, accessToken, refreshToken string) (string, error)
func (sm *SessionManager) ValidateToken(token string) (*Session, error)
func (sm *SessionManager) RevokeToken(token string)
func (sm *SessionManager) GetAccessToken(sessionToken string) (string, error)
func (sm *SessionManager) RefreshSession(sessionToken string) error
func (sm *SessionManager) GetSession(sessionToken string) (*Session, error)
```

### session.go
```go
func (s *Session) IsExpired() bool
func (s *Session) IsValid() bool
func generateToken() (string, error)
func NewSession(username, tenantID, accessToken, refreshToken string) (*Session, error)
func ValidateSession(session *Session) error
```

### token_store.go
```go
func NewFileTokenStore(filePath string, logger *logrus.Logger) *FileTokenStore
func (ts *FileTokenStore) Save(session *Session) error
func (ts *FileTokenStore) Load() (*Session, error)
func (ts *FileTokenStore) Delete() error
func (ts *FileTokenStore) Exists() bool
func (ts *FileTokenStore) GetFilePath() string
```

### cleanup.go
```go
func (sm *SessionManager) loadPersistedToken()
func (sm *SessionManager) cleanupExpiredSessions()
func (sm *SessionManager) cleanupOnce()
func (sm *SessionManager) StartCleanupRoutine(ctx context.Context)
```
