// Package acceptancetest owns the Temporal SDK lifecycle needed by local
// acceptance tests while keeping SDK types inside the orchestration adapter.
package acceptancetest

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	imageagenttemporal "task-processor/internal/imageagent/temporal"
	platformtemporal "task-processor/internal/platform/temporal"
)

// Driver owns one real Temporal client and, while started, one Organization
// worker. It intentionally exposes no Temporal SDK types to its callers.
type Driver struct {
	mu          sync.Mutex
	client      sdkclient.Client
	closeClient func() error
	worker      sdkworker.Worker
}

// Connect retries a real Temporal connection until ctx expires.
func Connect(ctx context.Context, address string) (*Driver, error) {
	var lastErr error
	for ctx.Err() == nil {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		client, closeClient, err := platformtemporal.Dial(attemptCtx, platformtemporal.Config{Address: address, Namespace: "default"})
		cancel()
		if err == nil {
			return &Driver{client: client, closeClient: closeClient}, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
		case <-time.After(250 * time.Millisecond):
		}
	}
	if lastErr == nil {
		lastErr = ctx.Err()
	}
	return nil, fmt.Errorf("connect to real Temporal server: %w", lastErr)
}

// OrganizationClient returns the current Organization workflow adapter.
func (driver *Driver) OrganizationClient() *imageagenttemporal.Client {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return imageagenttemporal.NewOrganizationClient(driver.client)
}

// StartOrganizationWorker starts the actual Organization worker wiring.
func (driver *Driver) StartOrganizationWorker(activities *imageagenttemporal.Activities) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.worker != nil {
		return fmt.Errorf("organization worker is already started")
	}
	worker, err := imageagenttemporal.NewWorker(imageagenttemporal.WorkerConfig{
		Client: driver.client, Activities: activities, WireMode: imageagenttemporal.WorkerWireModeOrganization,
	})
	if err != nil {
		return err
	}
	if err := worker.Start(); err != nil {
		return err
	}
	driver.worker = worker
	return nil
}

// StopWorker synchronously stops the owned worker and may be called repeatedly.
func (driver *Driver) StopWorker() {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.worker != nil {
		driver.worker.Stop()
		driver.worker = nil
	}
}

// Close stops the worker and closes the SDK client exactly once.
func (driver *Driver) Close() error {
	driver.StopWorker()
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closeClient == nil {
		return nil
	}
	closeClient := driver.closeClient
	driver.closeClient = nil
	return closeClient()
}
