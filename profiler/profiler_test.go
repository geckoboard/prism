package profiler

import (
	"sync"
	"testing"
	"time"
)

func TestProfiler(t *testing.T) {
	nestedCallName := "func2"

	sink := newBufferedSink()
	Init(sink, "profiler-test")

	BeginProfile("func1")
	<-time.After(5 * time.Millisecond)

	// Invoke EndProfile/Enter/Leave from a different go-routine. These
	// calls should be ignored as there is no active profile running in
	// the go-routine.
	var wg sync.WaitGroup
	wg.Add(2)
	for depth := 0; depth < 2; depth++ {
		go func(depth int) {
			defer wg.Done()
			if depth == 0 {
				// Calling EndProfile should not interfere with the
				// profile data collected by the other goroutine
				defer EndProfile()
			} else {
				Enter(nestedCallName)
				defer Leave()
			}
		}(depth)
	}

	Enter(nestedCallName)
	<-time.After(10 * time.Millisecond)
	Leave()

	wg.Wait()
	EndProfile()

	// Shutdown and flush sink
	Shutdown()

	expEntries := 1
	if len(sink.buffer) != expEntries {
		t.Fatalf("expected sink to capture %d entries; got %d", expEntries, len(sink.buffer))
	}

	profile := sink.buffer[0]

	expID := threadID()
	if profile.ID != expID {
		t.Fatalf("expected profile ID to be %d; got %d", expID, profile.ID)
	}

	expInvocations := 1
	if len(profile.Target.NestedCalls) != expInvocations {
		t.Fatalf("expected profile target func1 to capture %d unique nested calls; got %d", expInvocations, len(profile.Target.NestedCalls))
	}

	nestedCall := profile.Target.NestedCalls[0]
	if nestedCall.FnName != nestedCallName {
		t.Fatalf("expected nested call name to be %q; got %q", nestedCallName, nestedCall.FnName)
	}

	if nestedCall.Invocations != expInvocations {
		t.Fatalf("expected nested call %q to be invoked %d times; got %d", nestedCallName, expInvocations, nestedCall.Invocations)
	}
}

type bufferedSink struct {
	sigChan   chan struct{}
	inputChan chan *Profile
	buffer    []*Profile
}

func newBufferedSink() *bufferedSink {
	return &bufferedSink{
		sigChan: make(chan struct{}, 0),
		buffer:  make([]*Profile, 0),
	}
}

func (s *bufferedSink) Open(_ int) error {
	s.inputChan = make(chan *Profile, 0)
	go s.worker()
	<-s.sigChan
	return nil
}

// Shutdown the sink.
func (s *bufferedSink) Close() error {
	// Signal worker to exit and wait for confirmation
	close(s.inputChan)
	<-s.sigChan
	close(s.sigChan)
	return nil
}

// Get a channel for piping profile entries to the sink.
func (s *bufferedSink) Input() chan<- *Profile {
	return s.inputChan
}

func (s *bufferedSink) worker() {
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
		s.buffer = append(s.buffer, profile)
	}
}
