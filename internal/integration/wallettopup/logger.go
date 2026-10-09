package wallettopup

import "task-processor/internal/integration/paymentsecurity"

// SDK errors can contain raw signed provider payloads. The caller records only
// bounded safe error codes, never the SDK's request/response diagnostics.
type providerSilentLogger = paymentsecurity.SilentLogger
