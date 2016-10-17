package profiler

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type Entry struct {
	ThreadId uint64

	Name  string `json:"name"`
	Depth int    `json:"depth"`

	EnteredAt time.Time `json:"entered_at"`

	// Metrics for aggregated calls
	MinTime     time.Duration `json:"min_time"`
	MaxTime     time.Duration `json:"max_time"`
	TotalTime   time.Duration `json:"total_time"`
	Invocations int           `json:"invocations"`

	TotalProfileOverhead time.Duration `json:"-"`

	Children []*Entry `json:"children"`
	Parent   *Entry   `json:"-"`
}

var totalAllocs uint64

// Use a pool to reduce the number of entry allocations
var entryPool = sync.Pool{
	New: func() interface{} {
		atomic.AddUint64(&totalAllocs, 1)
		return &Entry{}
	},
}

// Allocate and initialize a new profile entry.
func makeEntry(name string, depth int) *Entry {
	entry := entryPool.Get().(*Entry)
	entry.Name = name
	entry.Depth = depth
	entry.MinTime = time.Duration(math.MaxInt64)
	entry.MaxTime = 0
	entry.TotalTime = 0
	entry.Invocations = 0
	entry.TotalProfileOverhead = 0

	entry.Children = make([]*Entry, 0)
	entry.Parent = nil

	return entry
}

// Update profile entry time stats taking into account profiler overhead.
func (pe *Entry) updateStats(overhead time.Duration) {
	elapsedTime := time.Since(pe.EnteredAt) - overhead
	pe.Invocations++
	if elapsedTime < pe.MinTime {
		pe.MinTime = elapsedTime
	}
	if elapsedTime > pe.MaxTime {
		pe.MaxTime = elapsedTime
	}
	pe.TotalTime += elapsedTime
	pe.TotalProfileOverhead += overhead
}

// Recursively subtract aggregated child node overhead from parent's total time.
func (pe *Entry) subtractOverhead() {
	var childOverhead time.Duration = 0
	for _, child := range pe.Children {
		childOverhead += child.TotalProfileOverhead
	}

	pe.TotalTime -= childOverhead
}

// Recursively free profile entries.
func (pe *Entry) Free() {
	// Free children first
	for _, child := range pe.Children {
		child.Free()
	}

	// Clear our children slice and return entry to the pool
	pe.Children = nil
	entryPool.Put(pe)
}
