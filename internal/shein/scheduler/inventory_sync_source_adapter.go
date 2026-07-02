package scheduler

import "task-processor/internal/shein/inventory"

type sheinSyncedInventoryProductSourceProvider interface {
	GetSheinSyncedInventoryProductSource() inventory.SyncedInventoryProductSource
}
