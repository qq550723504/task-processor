package sourcing

// SourceResult is the provider-neutral result of adapting one source record.
// Concrete crawler payloads and result types remain in their source adapters.
type SourceResult struct {
	Envelope SourceEnvelope
	Error    error
}
