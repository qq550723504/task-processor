package listingkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/core/logger"
	sheinproduct "task-processor/internal/shein/api/product"
)

func dumpSheinSubmitPayloadForDebug(taskID, action, requestID, stage string, product *sheinproduct.Product) {
	dir := resolveSheinSubmitDebugDumpDir()
	if dir == "" || product == nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.GetGlobalLogger("listingkit/submit").Warnf("failed to create SHEIN submit debug dump dir %s: %v", dir, err)
		return
	}
	payload, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		logger.GetGlobalLogger("listingkit/submit").Warnf("failed to marshal SHEIN submit debug payload for task=%s stage=%s: %v", taskID, stage, err)
		return
	}
	filename := fmt.Sprintf(
		"%s-%s-%s-%s-%s.json",
		time.Now().Format("20060102-150405"),
		sanitizeDebugFileToken(taskID),
		sanitizeDebugFileToken(action),
		sanitizeDebugFileToken(requestID),
		sanitizeDebugFileToken(stage),
	)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		logger.GetGlobalLogger("listingkit/submit").Warnf("failed to write SHEIN submit debug dump %s: %v", path, err)
		return
	}
	logger.GetGlobalLogger("listingkit/submit").Infof("dumped SHEIN submit payload for task=%s action=%s stage=%s path=%s", taskID, action, stage, path)
}

func resolveSheinSubmitDebugDumpDir() string {
	return strings.TrimSpace(os.Getenv("LISTINGKIT_DEBUG_SUBMIT_DUMP_DIR"))
}

func sanitizeDebugFileToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "empty"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		}
		if b.Len() >= 48 {
			break
		}
	}
	if b.Len() == 0 {
		return "empty"
	}
	return b.String()
}
