package consumer

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/redisclient"

	"github.com/sirupsen/logrus"
)

const storeQueueOwnerNodeKey = "listing:queue:owner:node:%s"

// StoreAssignmentProvider resolves the store IDs owned by a specific task node.
type StoreAssignmentProvider interface {
	GetOwnedStores(ctx context.Context, nodeID string) ([]int64, error)
	Close() error
}

// RedisStoreAssignmentProvider reads dynamic store ownership from Redis.
type RedisStoreAssignmentProvider struct {
	client *redisclient.Client
	logger *logrus.Logger
}

func NewRedisStoreAssignmentProvider(cfg *config.RedisConfig, logger *logrus.Logger) (*RedisStoreAssignmentProvider, error) {
	client, err := redisclient.New(cfg)
	if err != nil {
		return nil, err
	}
	return &RedisStoreAssignmentProvider{
		client: client,
		logger: logger,
	}, nil
}

func (p *RedisStoreAssignmentProvider) GetOwnedStores(ctx context.Context, nodeID string) ([]int64, error) {
	trimmedNodeID := strings.TrimSpace(nodeID)
	if trimmedNodeID == "" {
		return nil, fmt.Errorf("nodeID is empty")
	}

	members, err := p.client.SMembers(ctx, fmt.Sprintf(storeQueueOwnerNodeKey, trimmedNodeID))
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []int64{}, nil
	}

	storeIDs := make([]int64, 0, len(members))
	for _, member := range members {
		storeID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
		if parseErr != nil {
			if p.logger != nil {
				p.logger.WithFields(logrus.Fields{
					"node_id": nodeID,
					"value":   member,
				}).Warnf("skip invalid dynamic owned store ID: %v", parseErr)
			}
			continue
		}
		storeIDs = append(storeIDs, storeID)
	}
	sort.Slice(storeIDs, func(i, j int) bool {
		return storeIDs[i] < storeIDs[j]
	})
	return storeIDs, nil
}

func (p *RedisStoreAssignmentProvider) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}
