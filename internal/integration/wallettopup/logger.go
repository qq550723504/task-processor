package wallettopup

// SDK errors can contain raw signed provider payloads. The caller records only
// bounded safe error codes, never the SDK's request/response diagnostics.
type providerSilentLogger struct{}

func (providerSilentLogger) Debug(...any)          {}
func (providerSilentLogger) Info(...any)           {}
func (providerSilentLogger) Warn(...any)           {}
func (providerSilentLogger) Error(...any)          {}
func (providerSilentLogger) Debugf(string, ...any) {}
func (providerSilentLogger) Infof(string, ...any)  {}
func (providerSilentLogger) Warnf(string, ...any)  {}
func (providerSilentLogger) Errorf(string, ...any) {}
