package profiler

import (
	"sync"
	"testing"
	"time"
)

func TestProfiler(t *testing.T) {
	type fnCall struct {
		Depth    int
		Name     string
		WorkTime time.Duration
		Calls    []fnCall
	}

	callgraph := fnCall{
		Name:     "func1",
		WorkTime: 10 * time.Millisecond,
		Calls: []fnCall{
			fnCall{
				Depth:    1,
				Name:     "func2",
				WorkTime: 10 * time.Millisecond,
			},
			fnCall{
				Depth:    1,
				Name:     "func3",
				WorkTime: 5 * time.Millisecond,
			},
			fnCall{
				Depth:    1,
				Name:     "func3",
				WorkTime: 5 * time.Millisecond,
			},
		},
	}

	var simulateCalls func(call fnCall)
	simulateCalls = func(call fnCall) {
		if call.Depth == 0 {
			BeginProfile(call.Name)
			defer EndProfile()
		} else {
			Enter(call.Name)
			defer Leave()
		}

		// Exec the same enter/leave flow in a separate goroutine. This should not
		// affect the current profile as it uses a different goid
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if call.Depth == 0 {
				// Calling EndProfile should not interfere with the
				// profile data collected by the other goroutine
				defer EndProfile()
			} else {
				Enter(call.Name)
				defer Leave()
			}
		}()
		wg.Wait()

		// Simulate work
		<-time.After(call.WorkTime)

		// Follow call graph
		if call.Calls == nil {
			return
		}

		for _, nextCall := range call.Calls {
			simulateCalls(nextCall)
		}
	}

	sink := newBufferedSink()
	Init(sink, "profiler-test")
	simulateCalls(callgraph)
	Shutdown()

	expEntries := 1
	if len(sink.buffer) != expEntries {
		t.Fatalf("expected sink to capture %d entries; got %d", expEntries, len(sink.buffer))
	}

	pe := sink.buffer[0]

	expNestedCalls := 2
	if len(pe.Children) != expNestedCalls {
		t.Fatalf("expected nested call count to be %d; got %d", expNestedCalls, len(pe.Children))
	}

	expInvocations := 1
	if pe.Invocations != expInvocations {
		t.Fatalf("expected func1 invocation count to be %d; got %d", expInvocations, pe.Invocations)
	}

	if pe.TotalTime < pe.Children[0].TotalTime+pe.Children[1].TotalTime+callgraph.WorkTime {
		t.Fatalf("expected func1 total time to be >= the sum of the nested call total time %v + func1 work time %v; got %v", pe.Children[0].TotalTime+pe.Children[1].TotalTime, callgraph.WorkTime, pe.TotalTime)
	}

	if pe.TotalTime < pe.Children[0].TotalTime+pe.Children[1].TotalTime+callgraph.WorkTime {
		t.Fatalf("expected func1 total time to be >= the sum of the nested call total time %v + func1 work time %v; got %v", pe.Children[0].TotalTime+pe.Children[1].TotalTime, callgraph.WorkTime, pe.TotalTime)
	}

	expInvocations = 2
	if pe.Children[1].Invocations != expInvocations {
		t.Fatalf("expected invocation count for func 3 to be %d; got %d", expInvocations, pe.Children[1].Invocations)
	}

	pe.Free()
	if pe.Children != nil {
		t.Fatal("expected entry children to be nil after invoking Free")
	}
}

type bufferedSink struct {
	sigChan   chan struct{}
	inputChan chan *Entry
	buffer    []*Entry
}

func newBufferedSink() *bufferedSink {
	return &bufferedSink{
		sigChan: make(chan struct{}, 0),
		buffer:  make([]*Entry, 0),
	}
}

func (s *bufferedSink) Open(_ int) error {
	s.inputChan = make(chan *Entry, 0)
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
func (s *bufferedSink) Input() chan<- *Entry {
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
