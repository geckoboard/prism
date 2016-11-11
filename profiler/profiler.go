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

	// We maintain a dedicated call stack for each profiled goroutine. Each
	// map entry points to the currently entered function scope.
	activeProfiles map[uint64]*fnCall

	// A sink for emitted profile entries.
	outputSink Sink
)

// Time is an alias to time.Time(). It is exposed by this package so our injected
// code does not need to import the time package which may cause conflict with
// other imported/aliased packages.
func Time() time.Time {
	return time.Now()
}

// Init handles the initialization of the prism profiler. This method must be
// called before invoking any other method from this package.
func Init(sink Sink, capturedProfileLabel string) {
	err := sink.Open(defaultSinkBufferSize)
	if err != nil {
		err = fmt.Errorf("profiler: error initializing sink: %s", err)
		panic(err)
	}

	outputSink = sink
	activeProfiles = make(map[uint64]*fnCall, 0)
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
func BeginProfile(rootFnName string, at time.Time) {
	tick := time.Now()

	tid := threadID()

	rootCall := makeFnCall(rootFnName)
	rootCall.enteredAt = at

	profileMutex.Lock()
	activeProfiles[tid] = rootCall
	profileMutex.Unlock()

	rootCall.profilerOverhead += time.Since(tick)
}

// EndProfile finalizes and ships a currently active profile.
func EndProfile() {
	tick := time.Now()
	tid := threadID()

	profileMutex.Lock()
	rootCall := activeProfiles[tid]
	if rootCall == nil {
		// No active profile for this threadID; skip
		profileMutex.Unlock()
		return
	}

	delete(activeProfiles, tid)
	profileMutex.Unlock()

	// Generate profile
	rootCall.exitedAt = time.Now()
	rootCall.profilerOverhead += time.Since(tick)
	profile := genProfile(tid, profileLabel, rootCall)
	rootCall.free()

	// Ship profile
	outputSink.Input() <- profile
}

// Enter adds a new nested function call to the profile linked to the current go-routine ID.
func Enter(fnName string, at time.Time) {
	tick := time.Now()
	tid := threadID()

	profileMutex.Lock()
	parentCall := activeProfiles[tid]
	if parentCall == nil {
		// No active profile for this threadID; skip
		profileMutex.Unlock()
		return
	}

	call := makeFnCall(fnName)
	call.enteredAt = at
	parentCall.nestCall(call)

	activeProfiles[tid] = call
	profileMutex.Unlock()

	// Update overhead estimate
	call.profilerOverhead += time.Since(tick)
}

// Leave exits the current function in the profile linked to the current go-routine ID.
func Leave() {
	tick := time.Now()
	tid := threadID()

	profileMutex.Lock()
	call := activeProfiles[tid]
	if call == nil {
		// No active profile for this threadID; skip
		profileMutex.Unlock()
		return
	}

	if call.parent == nil {
		profileMutex.Unlock()
		panic(fmt.Sprintf("profiler: [BUG] attempted to exit an active profile (tid %d)", tid))
	}

	// Exit current scope
	activeProfiles[tid] = call.parent
	profileMutex.Unlock()

	// Update exit timestamp and overhead estimate
	call.exitedAt = time.Now()
	call.profilerOverhead += time.Since(tick)
	call.parent.profilerOverhead += call.profilerOverhead
}
