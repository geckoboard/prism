package sink

import "github.com/geckoboard/prism/profiler"

type discardSink struct {
	sigChan      chan struct{}
	inputChan    chan *profiler.Profile
	numDiscarded int
}

// NewDiscardSink creates a profile entry sink instance which discards all
// incoming profile entries.
func NewDiscardSink() profiler.Sink {
	return &discardSink{
		sigChan: make(chan struct{}, 0),
	}
}

// Initialize the sink.
func (s *discardSink) Open(inputBufferSize int) error {
	s.inputChan = make(chan *profiler.Profile, inputBufferSize)

	// start worker and wait for ready signal
	go s.worker()
	<-s.sigChan
	return nil
}

// Shutdown the sink.
func (s *discardSink) Close() error {
	// Signal worker to exit and wait for confirmation
	close(s.inputChan)
	<-s.sigChan
	close(s.sigChan)
	return nil
}

// Get a channel for piping profile entries to the sink.
func (s *discardSink) Input() chan<- *profiler.Profile {
	return s.inputChan
}

func (s *discardSink) worker() {
	// Signal that worker has started
	s.sigChan <- struct{}{}
	defer func() {
		// Signal that we have stopped
		s.sigChan <- struct{}{}
	}()

	for {
		profile, sinkOpen := <-s.inputChan
		if !sinkOpen {
			return
		}
		s.numDiscarded++
	}
}
