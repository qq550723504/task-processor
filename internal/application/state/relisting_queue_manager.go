package state

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// ReListingQueueManager 重新上架队列管理器（内存版）
type ReListingQueueManager struct {
	queues map[string][]string // key: tenantID:shopID, value: task data list
	mutex  sync.RWMutex
}

// NewReListingQueueManager 创建重新上架队列管理器
func NewReListingQueueManager() *ReListingQueueManager {
	return &ReListingQueueManager{
		queues: make(map[string][]string),
	}
}

// PushTask 添加任务到队列头部
func (m *ReListingQueueManager) PushTask(tenantID, shopID int64, taskData string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)

	// 添加到队列头部
	m.queues[key] = append([]string{taskData}, m.queues[key]...)

	logrus.Infof("任务已添加到重新上架队列: 租户=%d, 店铺=%d, 队列长度=%d", tenantID, shopID, len(m.queues[key]))
}

// PopTask 从队列尾部取出任务
func (m *ReListingQueueManager) PopTask(tenantID, shopID int64) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	queue, exists := m.queues[key]

	if !exists || len(queue) == 0 {
		return "", fmt.Errorf("队列为空")
	}

	// 从尾部取出
	taskData := queue[len(queue)-1]
	m.queues[key] = queue[:len(queue)-1]

	// 如果队列为空，删除键
	if len(m.queues[key]) == 0 {
		delete(m.queues, key)
	}

	return taskData, nil
}

// GetQueueLength 获取队列长度
func (m *ReListingQueueManager) GetQueueLength(tenantID, shopID int64) int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	if queue, exists := m.queues[key]; exists {
		return int64(len(queue))
	}

	return 0
}

// GetAllKeys 获取所有队列键
func (m *ReListingQueueManager) GetAllKeys() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	keys := make([]string, 0, len(m.queues))
	for key := range m.queues {
		keys = append(keys, key)
	}

	return keys
}

// ClearQueue 清空指定队列
func (m *ReListingQueueManager) ClearQueue(tenantID, shopID int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	delete(m.queues, key)

	logrus.Infof("重新上架队列已清空: 租户=%d, 店铺=%d", tenantID, shopID)
}
