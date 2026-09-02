package httpapi

import (
	"errors"
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
