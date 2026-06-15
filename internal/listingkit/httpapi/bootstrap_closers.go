package httpapi

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

type closerStack struct {
	items []func() error
}

func (s *closerStack) Add(items ...func() error) {
	for _, item := range items {
		if item != nil {
			s.items = append(s.items, item)
		}
	}
}

func (s *closerStack) Snapshot() []func() error {
	return append([]func() error{}, s.items...)
}

func (s *closerStack) Close() error {
	var closeErr error
	for i := len(s.items) - 1; i >= 0; i-- {
		if err := s.items[i](); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

func buildNamedWithClosers[T any](name string, builder func(*config.Config, *logrus.Logger) (T, []func() error, error), cfg *config.Config, logger *logrus.Logger, closers *closerStack) (T, error) {
	startedAt := time.Now()
	if logger != nil {
		logger.WithField("component", "listingkit/httpapi").WithField("repository", name).Info("listingkit repository build begin")
	}
	value, err := buildWithClosers(builder, cfg, logger, closers)
	if err != nil {
		if logger != nil {
			logger.WithError(err).WithField("component", "listingkit/httpapi").WithField("repository", name).Warn("listingkit repository build failed")
		}
		var zero T
		return zero, err
	}
	if logger != nil {
		logger.WithField("component", "listingkit/httpapi").WithField("repository", name).WithField("elapsed", time.Since(startedAt)).Info("listingkit repository build done")
	}
	return value, nil
}
