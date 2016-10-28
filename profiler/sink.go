package profiler

// Sink defines an interface for processing profile entries emitted by the profiler.
type Sink interface {
	// Initialize the sink input channel with the specified buffer capacity.
	Open(inputBufferSize int) error

	// Shutdown the sink.
	Close() error

	// Get a channel for piping profile entries to the sink.
	Input() chan<- *Entry
}
