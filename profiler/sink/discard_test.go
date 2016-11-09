package sink

import (
	"testing"

	"github.com/geckoboard/prism/profiler"
)

func TestDiscardSink(t *testing.T) {
	s := NewDiscardSink()
	err := s.Open(1)
	if err != nil {
		t.Fatal(err)
	}

	numEntries := 5
	for i := 0; i < numEntries; i++ {
		s.Input() <- &profiler.Profile{}
	}

	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	numDiscarded := s.(*discardSink).numDiscarded
	if numDiscarded != numEntries {
		t.Errorf("expected sink discarded entry count to be %d; got %d", numEntries, numDiscarded)
	}
}
