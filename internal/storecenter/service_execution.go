package storecenter

import (
	"context"
	"time"
)

type ServiceCommand string

const (
	ServiceCommandActivate   ServiceCommand = "store_service_activate"
	ServiceCommandRenew      ServiceCommand = "store_service_renew"
	ServiceCommandReactivate ServiceCommand = "store_service_reactivate"
)

type ServiceExecution struct {
	OrganizationID        string
	OperationID           string
	StoreID               string
	Command               ServiceCommand
	Quantity              int64
	MaxQuantity           int64
	ExpectedStoreVersion  int64
	ExpectedConnectionRef string
	ConnectionStatus      ConnectionStatus
	ActorSubject          string
	OccurredAt            time.Time
	RequestFingerprint    string
}

type ServiceReplay struct {
	OrganizationID     string
	OperationID        string
	RequestFingerprint string
}

type ServiceOperationSnapshot struct {
	OrganizationID string            `json:"organization_id"`
	OperationID    string            `json:"operation_id"`
	StoreID        string            `json:"store_id"`
	Command        ServiceCommand    `json:"command"`
	Quantity       string            `json:"quantity"`
	ResourceType   string            `json:"resource_type"`
	BalanceAfter   string            `json:"balance_after"`
	StoreVersion   int64             `json:"store_version"`
	ServiceState   StoreServiceState `json:"service_state"`
	EventID        string            `json:"event_id"`
}

type ServiceOperationResult struct {
	Snapshot ServiceOperationSnapshot
	Replayed bool
}

type ServiceLifecycleExecutor interface {
	ReplayServiceLifecycle(context.Context, ServiceReplay) (ServiceOperationResult, bool, error)
	ExecuteServiceLifecycle(context.Context, ServiceExecution) (ServiceOperationResult, error)
}
