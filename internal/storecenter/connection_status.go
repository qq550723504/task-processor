package storecenter

import (
	"context"
	"time"
)

type ConnectionStatus string

const (
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	ConnectionStatusConnected    ConnectionStatus = "connected"
	ConnectionStatusExpired      ConnectionStatus = "expired"
	ConnectionStatusUnavailable  ConnectionStatus = "unavailable"
)

// ConnectionStatusInput is the narrow provider boundary. ConnectionRef is an
// opaque lookup reference and is never a Store Center response field.
type ConnectionStatusInput struct {
	OrganizationID string   `json:"-"`
	StoreID        string   `json:"storeId"`
	Platform       Platform `json:"platform"`
	ConnectionRef  string   `json:"-"`
}

type ConnectionStatusProvider interface {
	Status(context.Context, ConnectionStatusInput) (ConnectionStatus, error)
}

func resolveConnectionStatus(ctx context.Context, provider ConnectionStatusProvider, input ConnectionStatusInput, timeout time.Duration) ConnectionStatus {
	if input.ConnectionRef == "" {
		return ConnectionStatusDisconnected
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	status, err := provider.Status(callCtx, input)
	if err != nil {
		return ConnectionStatusUnavailable
	}
	switch status {
	case ConnectionStatusConnected, ConnectionStatusExpired, ConnectionStatusUnavailable:
		return status
	default:
		return ConnectionStatusUnavailable
	}
}
