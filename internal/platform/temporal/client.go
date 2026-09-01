// Package temporal owns construction and lifetime of Temporal SDK clients.
package temporal

import (
	"context"
	"sync"

	sdkclient "go.temporal.io/sdk/client"
)

const (
	defaultAddress   = "localhost:7233"
	defaultNamespace = "default"
)

// Config contains business-neutral Temporal connection settings.
type Config struct {
	Address   string
	Namespace string
}

// Options converts platform configuration to Temporal SDK options.
func Options(config Config) sdkclient.Options {
	address := config.Address
	if address == "" {
		address = defaultAddress
	}
	namespace := config.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	return sdkclient.Options{HostPort: address, Namespace: namespace}
}

// Dial constructs a Temporal SDK client and returns its idempotent close owner.
func Dial(ctx context.Context, config Config) (sdkclient.Client, func() error, error) {
	return dialWith(ctx, config, sdkclient.DialContext)
}

type dialFunc func(context.Context, sdkclient.Options) (sdkclient.Client, error)

func dialWith(ctx context.Context, config Config, dial dialFunc) (sdkclient.Client, func() error, error) {
	client, err := dial(ctx, Options(config))
	if err != nil {
		return nil, nil, err
	}
	var once sync.Once
	return client, func() error {
		once.Do(client.Close)
		return nil
	}, nil
}
