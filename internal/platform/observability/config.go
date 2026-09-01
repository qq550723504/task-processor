package observability

// Config contains transport-level tracing settings owned by the application.
type Config struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
	Insecure    bool
}
