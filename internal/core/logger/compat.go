// Package logger provides deprecated compatibility aliases for platform logging.
// Deprecated: import task-processor/internal/platform/logging from runtime
// composition code. Legacy business packages should migrate to local logging
// ports in their owning domain phases.
package logger

import platformlogging "task-processor/internal/platform/logging"

type (
	LogManager      = platformlogging.LogManager
	LogConfig       = platformlogging.LogConfig
	LevelSplitHook  = platformlogging.LevelSplitHook
	LevelFileConfig = platformlogging.LevelFileConfig
	LoggerHelper    = platformlogging.LoggerHelper
)

const (
	FieldComponent  = platformlogging.FieldComponent
	FieldPlatform   = platformlogging.FieldPlatform
	FieldTaskID     = platformlogging.FieldTaskID
	FieldProductID  = platformlogging.FieldProductID
	FieldTenantID   = platformlogging.FieldTenantID
	FieldStoreID    = platformlogging.FieldStoreID
	FieldTraceID    = platformlogging.FieldTraceID
	FieldRequestID  = platformlogging.FieldRequestID
	FieldDurationMs = platformlogging.FieldDurationMs
	FieldRetryCount = platformlogging.FieldRetryCount
	FieldErrorCode  = platformlogging.FieldErrorCode
	FieldErrorType  = platformlogging.FieldErrorType
	FieldOperation  = platformlogging.FieldOperation
	FieldStatus     = platformlogging.FieldStatus
	FieldUserID     = platformlogging.FieldUserID
	FieldSessionID  = platformlogging.FieldSessionID
)

var (
	DefaultLogConfig    = platformlogging.DefaultLogConfig
	NewLogManager       = platformlogging.NewLogManager
	InitGlobalLogger    = platformlogging.InitGlobalLogger
	GetGlobalLogManager = platformlogging.GetGlobalLogManager
	GetGlobalLogger     = platformlogging.GetGlobalLogger
	SetGlobalLogLevel   = platformlogging.SetGlobalLogLevel
	NewLevelSplitHook   = platformlogging.NewLevelSplitHook
	NewLoggerHelper     = platformlogging.NewLoggerHelper
	WithComponent       = platformlogging.WithComponent
	WithPlatform        = platformlogging.WithPlatform
	WithTaskContext     = platformlogging.WithTaskContext
	WithStoreContext    = platformlogging.WithStoreContext
	ShouldLog           = platformlogging.ShouldLog
	WithLogger          = platformlogging.WithLogger
	FromContext         = platformlogging.FromContext
	WithTraceID         = platformlogging.WithTraceID
	GetTraceID          = platformlogging.GetTraceID
	WithRequestID       = platformlogging.WithRequestID
	GetRequestID        = platformlogging.GetRequestID
	WithFields          = platformlogging.WithFields
	WithField           = platformlogging.WithField
)
