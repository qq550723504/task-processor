package logger

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	platformlogging "task-processor/internal/platform/logging"
)

func TestCompatibilityFacadeSharesPlatformSingletonAndExports(t *testing.T) {
	config := DefaultLogConfig()
	config.Console = false
	InitGlobalLogger(config)
	t.Cleanup(func() { _ = GetGlobalLogManager().Close() })

	if got, want := GetGlobalLogManager(), platformlogging.GetGlobalLogManager(); got != want {
		t.Fatal("compatibility facade and platform logging do not share the same manager")
	}
	legacyEntry := GetGlobalLogger("compat")
	platformEntry := platformlogging.GetGlobalLogger("platform")
	if legacyEntry.Logger != platformEntry.Logger {
		t.Fatal("compatibility facade and platform logging do not share the same logger")
	}

	manager := NewLogManager(&LogConfig{Console: false})
	t.Cleanup(func() { _ = manager.Close() })
	var _ *LogManager = manager
	if err := SetGlobalLogLevel("debug"); err != nil {
		t.Fatal(err)
	}

	hook, err := NewLevelSplitHook([]LevelFileConfig(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	var _ *LevelSplitHook = hook
	var _ *LoggerHelper = NewLoggerHelper(legacyEntry)

	ctx := WithLogger(context.Background(), legacyEntry)
	ctx = WithTraceID(ctx, "trace")
	ctx = WithRequestID(ctx, "request")
	ctx = WithFields(ctx, logrus.Fields{"one": 1})
	ctx = WithField(ctx, "two", 2)
	if FromContext(ctx, "fallback").Logger != platformEntry.Logger {
		t.Fatal("context helper returned a logger from a different singleton")
	}
	if GetTraceID(ctx) != "trace" || GetRequestID(ctx) != "request" {
		t.Fatal("compatibility context helpers lost trace or request IDs")
	}

	_ = WithComponent("component")
	_ = WithPlatform("platform")
	_ = WithTaskContext(1, "product")
	_ = WithStoreContext(2, 3)
	_ = ShouldLog(0)

	fields := []string{
		FieldComponent,
		FieldPlatform,
		FieldTaskID,
		FieldProductID,
		FieldTenantID,
		FieldStoreID,
		FieldTraceID,
		FieldRequestID,
		FieldDurationMs,
		FieldRetryCount,
		FieldErrorCode,
		FieldErrorType,
		FieldOperation,
		FieldStatus,
		FieldUserID,
		FieldSessionID,
	}
	if len(fields) != 16 {
		t.Fatalf("compatibility field constants = %d, want 16", len(fields))
	}
}
