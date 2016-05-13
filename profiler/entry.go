package profiler

import (
	"math"
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

	Children []*Entry `json:"children"`
	Parent   *Entry   `json:"-"`
}

// Allocate and initialize a new profile entry.
func makeEntry(name string, depth int) *Entry {
	return &Entry{
		Name:  name,
		Depth: depth,

		Children: make([]*Entry, 0),

		EnteredAt: time.Now(),

		MinTime: time.Duration(math.MaxInt64),
		MaxTime: 0,
	}
}

// Update profile entry time stats.
func (pe *Entry) updateStats() {
	elapsedTime := time.Since(pe.EnteredAt)
	pe.Invocations++
	if elapsedTime < pe.MinTime {
		pe.MinTime = elapsedTime
	}
	if elapsedTime > pe.MaxTime {
		pe.MaxTime = elapsedTime
	}
	pe.TotalTime += elapsedTime
}
