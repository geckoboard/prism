package profiler

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestGroupMetrics(t *testing.T) {
	fnName := "testFn"
	numMetrics := 100

	metrics := metricsList{}
	for i := 0; i < numMetrics; i++ {
		metrics = append(
			metrics,
			&CallMetrics{
				FnName:    fnName,
				TotalTime: time.Duration(i) * time.Millisecond,
			},
		)
	}

	groupedMetric := metrics.aggregate()
	expValues := map[string]time.Duration{
		// This is an arithmetic series so S = n/2 * (a_0 + a_n)
		"total_time": time.Duration(numMetrics/2) * (metrics[0].TotalTime + metrics[numMetrics-1].TotalTime),
		//
		"min_time": 0,
		"max_time": metrics[numMetrics-1].TotalTime,
		//
		"mean_time":   metrics[0].TotalTime + metrics[numMetrics-1].TotalTime/2,
		"median_time": (metrics[(numMetrics/2)-1].TotalTime + metrics[numMetrics/2].TotalTime) / 2,
		//
		"p50_time": metrics[50-1].TotalTime,
		"p75_time": metrics[75-1].TotalTime,
		"p90_time": metrics[90-1].TotalTime,
		"p99_time": metrics[99-1].TotalTime,
	}

	dataDump, _ := json.Marshal(groupedMetric)
	var dataMap map[string]time.Duration
	json.Unmarshal(dataDump, &dataMap)
	for k, expVal := range expValues {
		if dataMap[k] != expVal {
			t.Errorf("expected group metric key %q to be %v; got %v", k, expVal, dataMap[k])
		}
	}

	// arithmetic series stddev: |d| * sqrt( (n-1)(n+1) / 12 )
	expStdDev := float64(time.Millisecond) * math.Sqrt(float64((numMetrics-1)*(numMetrics+1))/12.0)
	if groupedMetric.StdDev != expStdDev {
		t.Errorf("expected stddev to be %f; got %f", expStdDev, groupedMetric.StdDev)
	}

	// append one more entry to calc median time with odd number of metrics
	metrics = append(metrics, &CallMetrics{TotalTime: time.Duration(numMetrics) * time.Millisecond})
	groupedMetric = metrics.aggregate()
	expMedian := metrics[len(metrics)/2].TotalTime
	if groupedMetric.MedianTime != expMedian {
		t.Errorf("expected median time with odd number of metrics to be %v; got %v", expMedian, groupedMetric.MedianTime)
	}
}

func TestGenProfile(t *testing.T) {
	tick := time.Now()
	numNestedCalls := 10
	timeInRoot := 5 * time.Millisecond
	rootOverhead := 5 * time.Nanosecond
	nestedCallOverhead := 2 * time.Nanosecond

	root := &fnCall{
		fnName:           "func1",
		enteredAt:        tick,
		profilerOverhead: rootOverhead,
		nestedCalls:      make([]*fnCall, 0),
	}
	tick = tick.Add(rootOverhead + timeInRoot)

	var totalTimeInNestedCalls time.Duration
	for i := 0; i < numNestedCalls; i++ {
		timeInFunc := time.Duration(i+1) * time.Millisecond
		totalTimeInNestedCalls += timeInFunc
		exitedAt := tick.Add(timeInFunc + nestedCallOverhead)

		nestedCall := makeFnCall("func2")
		nestedCall.enteredAt = tick
		nestedCall.exitedAt = exitedAt
		nestedCall.profilerOverhead = nestedCallOverhead
		root.profilerOverhead += nestedCallOverhead
		root.nestCall(nestedCall)

		if nestedCall.parent != root {
			t.Fatal("expected nested call parent to be the root call after a call to nestCall()")
		}

		tick = exitedAt
	}
	root.exitedAt = tick.Add(root.profilerOverhead)
	root.profilerOverhead += root.profilerOverhead

	expLabel := "test-profile"
	expID := threadID()
	profile := genProfile(expID, expLabel, root)

	if profile.Label != expLabel {
		t.Fatalf("expected profile label to be %q; got %q", expLabel, profile.Label)
	}

	if profile.ID != expID {
		t.Fatalf("expected profile ID to be %d; got %d", expID, profile.ID)
	}

	if profile.CreatedAt != root.enteredAt {
		t.Fatal("expected profile creation timestamp to match the entry timestamp for the target func")
	}

	expRootTotalTime := root.exitedAt.Sub(root.enteredAt) - root.profilerOverhead
	if profile.Target.TotalTime != expRootTotalTime {
		t.Fatalf("expected func1 total time (sans any overhead) to be %d; got %d", expRootTotalTime, profile.Target.TotalTime)
	}

	expRootTotalTime = timeInRoot + totalTimeInNestedCalls
	if profile.Target.TotalTime != expRootTotalTime {
		t.Fatalf("expected func1 total time (sans any overhead) to be %d; got %d", expRootTotalTime, profile.Target.TotalTime)
	}

	nestedCalls := root.nestedCalls
	root.free()
	if root.nestedCalls != nil {
		t.Fatal("expected calling free() on the root fnCall to free the nestedCalls list")
	}
	for callIndex, nestedCall := range nestedCalls {
		if nestedCall.nestedCalls != nil {
			t.Errorf("[nested call %d] expected calling free() to set nestedCalls to nil", callIndex)
		}
	}
}

func TestCallGrouping(t *testing.T) {
	// Call graph models the following flow:
	//
	//       C--F
	//    B--|
	//    |  D
	// A--|
	//    |  C
	//    B--|
	//       E
	//
	graph := &fnCall{
		fnName: "A",
		nestedCalls: []*fnCall{
			&fnCall{
				fnName: "B",
				nestedCalls: []*fnCall{
					&fnCall{
						fnName: "C",
						nestedCalls: []*fnCall{
							&fnCall{
								fnName:      "F",
								nestedCalls: []*fnCall{},
							},
						},
					},
					&fnCall{
						fnName:      "D",
						nestedCalls: []*fnCall{},
					},
				},
			},
			&fnCall{
				fnName: "B",
				nestedCalls: []*fnCall{
					&fnCall{
						fnName:      "C",
						nestedCalls: []*fnCall{},
					},
					&fnCall{
						fnName:      "E",
						nestedCalls: []*fnCall{},
					},
				},
			},
		},
	}

	// Setup parent pointers
	var fillParents func(fn *fnCall)
	fillParents = func(fn *fnCall) {
		for _, child := range fn.nestedCalls {
			child.parent = fn
			fillParents(child)
		}
	}
	fillParents(graph)

	// Group calls
	cgt := &callGroupTree{
		levels: make([]*callGroups, 0),
	}

	cgt.insert(0, graph)
	cgt.linkGroups()

	// After grouping and linking we expect the processed tree to look like:
	//
	// [A] -- [B, B] --|- [C, C] -- [F]
	//                 |- [D]
	//                 |- [E]
	testGroup := cgt.levels[0].groups[0]
	if len(testGroup.calls) != 1 || testGroup.calls[0].fnName != "A" {
		t.Fatal(`expected call group at level 0 to contain 1 call entry of type "A"`)
	}
	if len(testGroup.nestedGroups) != 1 {
		t.Fatal(`expected call group at level 0 to contain 1 nested group`)
	}

	testGroup = testGroup.nestedGroups[0]
	if len(testGroup.calls) != 2 || testGroup.calls[0].fnName != "B" {
		t.Fatal(`expected call group at level 1 to contain 2 call entries of type "B"`)
	}
	if len(testGroup.nestedGroups) != 3 {
		t.Fatal(`expected call group at level 1 to contain 3 nested group`)
	}

	testSubGroup := testGroup.nestedGroups[0]
	if len(testSubGroup.calls) != 2 || testSubGroup.calls[0].fnName != "C" {
		t.Fatal(`expected call group 0 at level 2 to contain 2 call entries of type "C"`)
	}
	if len(testSubGroup.nestedGroups) != 1 {
		t.Fatal(`expected call group 0 at level 2 to contain 1 nested group`)
	}

	testSubGroup = testSubGroup.nestedGroups[0]
	if len(testSubGroup.calls) != 1 || testSubGroup.calls[0].fnName != "F" {
		t.Fatal(`expected call group at level 3 to contain 1 call entry of type "F"`)
	}
	if len(testSubGroup.nestedGroups) != 0 {
		t.Fatal(`expected call group at level 3 to contain 0 nested groups`)
	}

	testSubGroup = testGroup.nestedGroups[1]
	if len(testSubGroup.calls) != 1 || testSubGroup.calls[0].fnName != "D" {
		t.Fatal(`expected call group 1 at level 2 to contain 1 call entry of type "D"`)
	}
	if len(testSubGroup.nestedGroups) != 0 {
		t.Fatal(`expected call group 1 at level 2 to contain 0 nested groups`)
	}

	testSubGroup = testGroup.nestedGroups[2]
	if len(testSubGroup.calls) != 1 || testSubGroup.calls[0].fnName != "E" {
		t.Fatal(`expected call group 2 at level 2 to contain 1 call entry of type "E"`)
	}
	if len(testSubGroup.nestedGroups) != 0 {
		t.Fatal(`expected call group 2 at level 2 to contain 0 nested groups`)
	}
}
