package scheduler

import "task-processor/internal/shein/inventory"

type sheinSyncedInventoryProductFeedProvider interface {
	GetSheinSyncedInventoryProductFeed() inventory.SyncedInventoryProductFeed
}
