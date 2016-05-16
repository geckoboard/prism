package profiler

import (
	"fmt"
	"sync"
	"time"
)

const (
	shipChanBufSize = 100
)

var (
	// A mutex for protecting access to the activeProfiles map.
	profileMutex sync.Mutex

	// We maintain a set of active profiles grouped by goroutine id.
	activeProfiles map[uint64]*Entry

	// A buffered channel for shiping profiles
	shipChan chan *Entry

	// A channel for receiving start/term signals from shippers.
	shipSigChan chan struct{}
)

// Initialize profiler. This method must be called before invoking any other method from this package.
func Init() {
	activeProfiles = make(map[uint64]*Entry, 0)
	shipChan = make(chan *Entry, shipChanBufSize)
	shipSigChan = make(chan struct{}, 0)

	// run default shipper and wait for it to start
	go jsonProfileShipper()
	<-shipSigChan
}

// Wait for shippers to fully dequeue any buffered profiles and shut them down.
// This method should be called by main() before the program exits to ensure
// that no profile data is lost if the program executes too fast.
func Shutdown() {
	// Close shipChan and wait for shipper to exit
	close(shipChan)
	<-shipSigChan

	fmt.Printf("Prism allocs: %d\n\n", totalAllocs)
	close(shipSigChan)
}

// Create a new profile.
func BeginProfile(name string) {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()
	pe := makeEntry(name, 0)
	activeProfiles[tid] = pe
	pe.ThreadId = tid
	pe.EnteredAt = time.Now()
	pe.TotalProfileOverhead += time.Since(tick)
}

// End an active profile.
func EndProfile() {
	tick := time.Now()

	profileMutex.Lock()
	tid := threadId()
	profile, valid := activeProfiles[tid]
	delete(activeProfiles, tid)
	profileMutex.Unlock()

	if !valid {
		return
	}

	// Update profile stats and ship it
	profile.updateStats(time.Since(tick))
	profile.subtractOverhead()
	shipChan <- profile
}

// Enter a scope inside the currently active profile.
func Enter(name string) {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()

	profile, valid := activeProfiles[tid]
	if !valid {
		// Invoked through another call path that we do not monitor
		return
	}

	var pe *Entry = nil

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

// Exit a scope inside the currently active profile.
func Leave() {
	tick := time.Now()

	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()
	pe, valid := activeProfiles[tid]
	if !valid {
		// Invoked through another call path that we do not monitor
		return
	}

	if pe.Parent == nil {
		panic(fmt.Sprintf("profiler: [BUG] attempted to exit an active profile (tid %s)", tid))
	}

	// Pop parent
	activeProfiles[tid] = pe.Parent

	pe.updateStats(time.Since(tick))
}
