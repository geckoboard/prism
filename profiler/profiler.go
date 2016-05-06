package profiler

import (
	"fmt"
	"runtime"
	"strings"
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
	activeProfiles map[string]*Entry

	// A buffered channel for shiping profiles
	shipChan chan *Entry

	// A channel for receiving start/term signals from shippers.
	shipSigChan chan struct{}
)

// Initialize profiler. This method must be called before invoking any other method from this package.
func Init() {
	activeProfiles = make(map[string]*Entry, 0)
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
	close(shipSigChan)
}

// Create a new profile.
func BeginProfile(name string) {
	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()
	s := makeEntry(name, 0)
	s.ThreadId = tid
	activeProfiles[tid] = s
}

// End an active profile.
func EndProfile() {
	profileMutex.Lock()
	tid := threadId()
	profile, valid := activeProfiles[tid]
	delete(activeProfiles, tid)
	profileMutex.Unlock()

	if !valid {
		return
	}

	// Update profile stats and ship it
	profile.updateStats()
	shipChan <- profile
}

// Enter a scope inside the currently active profile.
func Enter(name string) {
	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()

	profile, valid := activeProfiles[tid]
	if !valid {
		panic(fmt.Sprintf("profiler: [BUG] entered scope %s (tid %s) without an active profile", name, tid))
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
}

// Exit a scope inside the currently active profile.
func Leave() {
	profileMutex.Lock()
	defer profileMutex.Unlock()

	tid := threadId()
	pe, valid := activeProfiles[tid]
	if !valid {
		panic(fmt.Sprintf("profiler: [BUG] attempted to exit scope (tid %s) without an active profile", tid))
	}

	if pe.Parent == nil {
		panic(fmt.Sprintf("profiler: [BUG] attempted to exit an active profile (tid %s)", tid))
	}

	pe.updateStats()

	// Pop parent
	activeProfiles[tid] = pe.Parent
}

// Detect the current go-routine id.
func threadId() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	return strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
}
