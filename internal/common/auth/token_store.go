package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TokenStore token持久化存储
type TokenStore struct {
	filePath string
	mutex    sync.RWMutex
}

// StoredToken 存储的token信息
type StoredToken struct {
	Token        string    `json:"token"`
	Username     string    `json:"username"`
	TenantID     string    `json:"tenant_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
}

// NewTokenStore 创建token存储
func NewTokenStore(filePath string) *TokenStore {
	return &TokenStore{
		filePath: filePath,
	}
}

// Save 保存token
func (ts *TokenStore) Save(token *StoredToken) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// 确保目录存在
	dir := filepath.Dir(ts.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(ts.filePath, data, 0600); err != nil {
		return err
	}

	logrus.Infof("Token已保存到: %s (用户: %s)", ts.filePath, token.Username)
	return nil
}

// Load 加载token
func (ts *TokenStore) Load() (*StoredToken, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	// 检查文件是否存在
	if _, err := os.Stat(ts.filePath); os.IsNotExist(err) {
		return nil, nil // 文件不存在，返回nil
	}

	// 读取文件
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return nil, err
	}

	// 反序列化
	var token StoredToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	// 检查是否过期
	if time.Now().After(token.ExpiresAt) {
		logrus.Infof("Token已过期 (用户: %s, 过期时间: %v)", token.Username, token.ExpiresAt)
		ts.mutex.RUnlock()
		ts.mutex.Lock()
		os.Remove(ts.filePath) // 删除过期token
		ts.mutex.Unlock()
		ts.mutex.RLock()
		return nil, nil
	}

	logrus.Infof("Token已加载: %s (用户: %s, 剩余有效期: %v)",
		ts.filePath, token.Username, time.Until(token.ExpiresAt).Round(time.Minute))
	return &token, nil
}

// Delete 删除token
func (ts *TokenStore) Delete() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if _, err := os.Stat(ts.filePath); os.IsNotExist(err) {
		return nil // 文件不存在
	}

	if err := os.Remove(ts.filePath); err != nil {
		return err
	}

	logrus.Infof("Token已删除: %s", ts.filePath)
	return nil
}

// Exists 检查token是否存在且有效
func (ts *TokenStore) Exists() bool {
	token, err := ts.Load()
	return err == nil && token != nil
}
