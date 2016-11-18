package profiler

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultSinkBufferSize = 100
	numCalibrationCalls   = 10000000
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

	// Function call invokation overhead; calculated by calibrate() and triggered by init()
	timeNowOverhead, timeSinceOverhead, deferredFnOverhead, fnCallOverhead time.Duration
)

func init() {
	calibrate()
}

// callibrate attempts to estimate the mean overhead for invoking time.Now(), time.Since(),
// as well as the mean time spent in function guard code (stack setup, pushing/popping
// registers, dealing with deferred calls e.t.c).
//
// Runtime overhead is generally in the nanosecond range but we need to
// properly account for it when calculating the total time spent inside a
// profiled function as it tends to skew our timing calculations when the profiled
// function is invoked a large number of times.
//
// To calculate an estimate, we time N executions of each function and then just
// calculate the mean execution time. As outliers may skew our results, using
// the median execution time would be better but calculating it is not a trivial
// operation due to the space and timing accurace required.
func calibrate() {
	// Benchmark function call time. We use a switch statement to ensure that
	// the compiler will not inline this function (see https://github.com/golang/go/issues/12312)
	fnCallBench := func(i int) {
		switch i {
		}
	}
	tick := time.Now()
	for i := 0; i < numCalibrationCalls; i++ {
		fnCallBench(i)
	}
	fnCallOverhead = time.Since(tick) / time.Duration(numCalibrationCalls)

	// Benchmark deferred function call time
	deferBench := func(i int) {
		defer func() { fnCallBench(i) }()
	}
	tick = time.Now()
	for i := 0; i < numCalibrationCalls; i++ {
		deferBench(i)
	}
	deferredFnOverhead = time.Since(tick) / time.Duration(numCalibrationCalls)

	// Benchmark time.Now()
	tick = time.Now()
	for i := 0; i < numCalibrationCalls; i++ {
		time.Now()
	}
	timeNowOverhead = fnCallOverhead + time.Since(tick)/time.Duration(numCalibrationCalls)

	// Benchmark time.Since()
	tick = time.Now()
	for i := 0; i < numCalibrationCalls; i++ {
		time.Since(tick)
	}
	timeSinceOverhead = fnCallOverhead + time.Since(tick)/time.Duration(numCalibrationCalls)
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
func BeginProfile(rootFnName string) {
	tick := time.Now()

	tid := threadID()

	rootCall := makeFnCall(rootFnName)
	rootCall.enteredAt = tick

	profileMutex.Lock()
	activeProfiles[tid] = rootCall
	profileMutex.Unlock()

	rootCall.profilerOverhead += timeNowOverhead + timeSinceOverhead + fnCallOverhead + time.Since(tick)
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

	// Generate profile;
	rootCall.exitedAt = time.Now()
	rootCall.profilerOverhead += 2*timeNowOverhead + timeSinceOverhead + deferredFnOverhead + time.Since(tick)
	profile := genProfile(tid, profileLabel, rootCall)
	rootCall.free()

	// Ship profile
	outputSink.Input() <- profile
}

// Enter adds a new nested function call to the profile linked to the current go-routine ID.
func Enter(fnName string) {
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
	call.enteredAt = tick
	parentCall.nestCall(call)

	activeProfiles[tid] = call
	profileMutex.Unlock()

	// Update overhead estimate
	call.profilerOverhead += timeNowOverhead + timeSinceOverhead + fnCallOverhead + time.Since(tick)
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

	// Update exit timestamp and overhead estimate for the parent. We also add in
	// an extra fnCallOverhead to account for the pointer dereferencing code for
	// updating the parent's overhead
	call.exitedAt = time.Now()
	call.profilerOverhead += 2*timeNowOverhead + timeSinceOverhead + deferredFnOverhead + 2*fnCallOverhead + time.Since(tick)
	call.parent.profilerOverhead += call.profilerOverhead
}
