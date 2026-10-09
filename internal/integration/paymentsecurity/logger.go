package paymentsecurity

// SDK errors can contain raw signed provider payloads. The caller records only
// bounded safe error codes, never the SDK's request/response diagnostics.
type SilentLogger struct{}

func (SilentLogger) Debug(...any)          {}
func (SilentLogger) Info(...any)           {}
func (SilentLogger) Warn(...any)           {}
func (SilentLogger) Error(...any)          {}
func (SilentLogger) Debugf(string, ...any) {}
func (SilentLogger) Infof(string, ...any)  {}
func (SilentLogger) Warnf(string, ...any)  {}
func (SilentLogger) Errorf(string, ...any) {}
