package profiler

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultSinkBufferSize = 100
)

var (
	// A mutex for protecting access to the activeProfiles map.
	profileMutex sync.Mutex

	// A label to be applied to generated profiles.
	profileLabel string

	// We maintain a set of active profiles grouped by goroutine id.
	activeProfiles map[uint64]*Entry

	// A sink for emitted profile entries.
	outputSink Sink
)

// Init handles the initialization of the prism profiler. This method must be
// called before invoking any other method from this package.
func Init(sink Sink, capturedProfileLabel string) {
	err := sink.Open(defaultSinkBufferSize)
	if err != nil {
		err = fmt.Errorf("profiler: error initializing sink: %s", err)
		panic(err)
	}

	outputSink = sink
	activeProfiles = make(map[uint64]*Entry, 0)
	profileLabel = capturedProfileLabel
}

// Shutdown waits for shippers to fully dequeue any buffered profiles and shuts
// them down. This method should be called by main() before the program exits
// to ensure that no profile data is lost if the program executes too fast.
func Shutdown() {
	err := outputSink.Close()
	if err != nil {
		err = fmt.Errorf("profiler: error shutting downg sink: %s", err)
		panic(err)
	}
}

// BeginProfile creates a new profile.
func BeginProfile(name string) {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadID()
	pe := makeEntry(name, 0)
	pe.Label = profileLabel
	activeProfiles[tid] = pe
	pe.ThreadID = tid
	pe.EnteredAt = time.Now()
	pe.TotalProfileOverhead += time.Since(tick)
}

// EndProfile finalizes and ships a currently active profile.
func EndProfile() {
	tick := time.Now()

	profileMutex.Lock()
	tid := threadID()
	profile, valid := activeProfiles[tid]
	delete(activeProfiles, tid)
	profileMutex.Unlock()

	if !valid {
		return
	}

	// Update profile stats and ship it
	profile.subtractOverhead()
	profile.updateStats(time.Since(tick))
	outputSink.Input() <- profile
}

// Enter appends a new scope inside the currently active profile.
func Enter(name string) {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadID()

	profile, valid := activeProfiles[tid]
	if !valid {
		// Invoked through another call path that we do not monitor
		return
	}

	var pe *Entry

	// Scan nested scopes from end to start looking for an existing match
	index := len(profile.Children) - 1
	for ; index >= 0; index-- {
		if profile.Children[index].Name == name {
			pe = profile.Children[index]
			break
		}
	}

	// First invocation
	if pe == nil {
		pe = makeEntry(name, profile.Depth+1)
		profile.Children = append(profile.Children, pe)
	}

	// Enter scope
	pe.Parent = profile
	pe.EnteredAt = time.Now()
	activeProfiles[tid] = pe
	pe.TotalProfileOverhead += time.Since(tick)
}

// Leave exits the inner-most scope inside the currently active profile.
func Leave() {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadID()
	pe, valid := activeProfiles[tid]
	if !valid {
		// Invoked through another call path that we do not monitor
		return
	}

	if pe.Parent == nil {
		panic(fmt.Sprintf("profiler: [BUG] attempted to exit an active profile (tid %d)", tid))
	}

	// Pop parent
	activeProfiles[tid] = pe.Parent

	pe.updateStats(time.Since(tick))
}
