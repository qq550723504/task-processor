// Package auth 提供认证功能
package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TokenStore token持久化存储接口
type TokenStore interface {
	Save(session *Session) error
	Load() (*Session, error)
	Delete() error
	Exists() bool
}

// FileTokenStore 文件token存储
type FileTokenStore struct {
	filePath string
	mutex    sync.RWMutex
	logger   *logrus.Logger
}

// NewFileTokenStore 创建文件token存储
func NewFileTokenStore(filePath string, logger *logrus.Logger) *FileTokenStore {
	return &FileTokenStore{
		filePath: filePath,
		logger:   logger,
	}
}

// Save 保存token
func (ts *FileTokenStore) Save(session *Session) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// 确保目录存在
	dir := filepath.Dir(ts.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(ts.filePath, data, 0600); err != nil {
		return err
	}

	ts.logger.Infof("Token已保存到: %s (用户: %s)", ts.filePath, session.Username)
	return nil
}

// Load 加载token
func (ts *FileTokenStore) Load() (*Session, error) {
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
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// 检查是否过期
	if session.IsExpired() {
		ts.logger.Infof("Token已过期 (用户: %s, 过期时间: %v)", session.Username, session.ExpiresAt)
		ts.deleteExpiredToken()
		return nil, nil
	}

	ts.logger.Infof("Token已加载: %s (用户: %s, 剩余有效期: %v)",
		ts.filePath, session.Username, time.Until(session.ExpiresAt).Round(time.Minute))
	return &session, nil
}

// Delete 删除token
func (ts *FileTokenStore) Delete() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if _, err := os.Stat(ts.filePath); os.IsNotExist(err) {
		return nil // 文件不存在
	}

	if err := os.Remove(ts.filePath); err != nil {
		return err
	}

	ts.logger.Infof("Token已删除: %s", ts.filePath)
	return nil
}

// Exists 检查token是否存在且有效
func (ts *FileTokenStore) Exists() bool {
	session, err := ts.Load()
	return err == nil && session != nil
}

// deleteExpiredToken 删除过期token（内部方法）
func (ts *FileTokenStore) deleteExpiredToken() {
	// 注意：这里不需要加锁，因为调用者已经持有读锁
	// 我们需要升级为写锁
	ts.mutex.RUnlock()
	ts.mutex.Lock()
	defer func() {
		ts.mutex.Unlock()
		ts.mutex.RLock()
	}()

	os.Remove(ts.filePath) // 删除过期token
}
