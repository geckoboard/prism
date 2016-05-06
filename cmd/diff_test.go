package cmd

import (
	"testing"

	"github.com/geckoboard/prism/profiler"
)

func TestCorrelateEntries(t *testing.T) {
	p1 := &profiler.Entry{
		Name:  "main",
		Depth: 0,
		Children: []*profiler.Entry{
			{
				Name:  "foo",
				Depth: 1,
				Children: []*profiler.Entry{
					{
						Name:  "bar",
						Depth: 2,
					},
				},
			},
		},
	}

	p2 := &profiler.Entry{
		Name:  "main",
		Depth: 0,
		Children: []*profiler.Entry{
			{
				Name:  "bar",
				Depth: 1,
			},
		},
	}

	out := correlateEntries([]*profiler.Entry{p1, p2})
	expLen := 3
	if len(out) != expLen {
		t.Fatalf("expected correlation map to contain %d entries; got %d", expLen, len(out))
	}

	expLen = 2
	if len(out["main"]) != expLen {
		t.Fatalf("expected correlation entry for main to contain %d entries; got %d", expLen, len(out["main"]))
	}

	expLen = 1
	if len(out["foo"]) != expLen {
		t.Fatalf("expected correlation entry for foo to contain %d entries; got %d", expLen, len(out["foo"]))
	}

	expLen = 2
	if len(out["bar"]) != expLen {
		t.Fatalf("expected correlation entry for bar to contain %d entries; got %d", expLen, len(out["bar"]))
	}
}
