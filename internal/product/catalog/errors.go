package catalog

import "errors"

var ErrInvalidSnapshot = errors.New("invalid product snapshot")

var (
	ErrInvalidPublication     = errors.New("invalid product snapshot publication")
	ErrPublicationConflict    = errors.New("product snapshot publication conflict")
	ErrSnapshotNotReady       = errors.New("product snapshot is not ready")
	ErrSnapshotTooLarge       = errors.New("product snapshot exceeds the read limit")
	ErrRepositoryUnavailable  = errors.New("product snapshot repository is unavailable")
	ErrRepositoryStateInvalid = errors.New("product snapshot repository state is invalid")
)

var ErrStaleSnapshot = errors.New("catalog base version is stale")
