package client

import (
	"fmt"
	"os"
	"path/filepath"

	"task-processor/internal/pkg/jsonx"
)

// AuthState 保存 SDS 登录后的本地鉴权信息。
type AuthState struct {
	AccessToken string `json:"accessToken"`
	OutToken    string `json:"outToken,omitempty"`
	MerchantID  int64  `json:"merchantId,omitempty"`
	UserID      int64  `json:"userId,omitempty"`
	Username    string `json:"username,omitempty"`
}

// AuthStateStore 负责 SDS 鉴权状态的本地持久化。
type AuthStateStore struct {
	filePath string
}

// NewAuthStateStore 创建鉴权状态存储。
func NewAuthStateStore(filePath string) *AuthStateStore {
	return &AuthStateStore{filePath: filePath}
}

// Load 读取本地鉴权状态。
func (s *AuthStateStore) Load() (*AuthState, error) {
	if s.filePath == "" {
		return nil, fmt.Errorf("auth file path is empty")
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read auth file: %w", err)
	}

	state := new(AuthState)
	if err := jsonx.UnmarshalBytes(data, state, "parse auth state"); err != nil {
		return nil, err
	}

	return state, nil
}

// Save 保存鉴权状态。
func (s *AuthStateStore) Save(state *AuthState) error {
	if s.filePath == "" {
		return fmt.Errorf("auth file path is empty")
	}
	if state == nil {
		return fmt.Errorf("auth state is nil")
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}

	data, err := jsonx.MarshalPretty(state)
	if err != nil {
		return fmt.Errorf("marshal auth state: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("write auth file: %w", err)
	}

	return nil
}

// Clear 删除本地鉴权状态。
func (s *AuthStateStore) Clear() error {
	if s.filePath == "" {
		return fmt.Errorf("auth file path is empty")
	}
	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove auth file: %w", err)
	}
	return nil
}
