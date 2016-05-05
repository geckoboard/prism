package profiler

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
)

type Entry struct {
	ThreadId string

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

func (pe *Entry) String() string {
	buf := bytes.NewBufferString("")
	if pe.Depth >= 0 {
		if pe.Depth > 0 {
			buf.WriteString(strings.Repeat("| ", pe.Depth))
			if len(pe.Children) == 0 {
				buf.WriteString("- ")
			} else {
				buf.WriteString("+ ")
			}
		}

		if pe.Invocations > 1 {
			buf.WriteString(
				fmt.Sprintf("%s [min %1.2f ms, avg %1.2f ms, max %1.2f ms, total %1.2f ms] [invocations: %d]\n",
					pe.Name,
					float64(pe.MinTime.Nanoseconds())/1.0e6,
					float64(pe.TotalTime.Nanoseconds())/float64(pe.Invocations*1e6),
					float64(pe.MaxTime.Nanoseconds())/1.0e6,
					float64(pe.TotalTime.Nanoseconds())/1.0e6,
					pe.Invocations,
				),
			)
		} else {
			buf.WriteString(
				fmt.Sprintf("%s [total %1.2f ms]\n",
					pe.Name,
					float64(pe.TotalTime.Nanoseconds())/float64(pe.Invocations*1e6),
				),
			)
		}
	}

	// Encode nested scopes
	for _, child := range pe.Children {
		buf.WriteString(child.String())
	}

	return buf.String()
}
