package profiler

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Profile wraps the processed metrics for a particular execution of a prism-hooked target.
type Profile struct {
	ID        uint64    `json:"-"`
	CreatedAt time.Time `json:"-"`

	Label  string       `json:"label"`
	Target *CallMetrics `json:"target"`
}

type metricsList []*CallMetrics

func (p metricsList) Len() int           { return len(p) }
func (p metricsList) Less(i, j int) bool { return p[i].TotalTime < p[j].TotalTime }
func (p metricsList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// aggregate iterates all CallMetrics in the list and returns a composite CallMetric
// representing the Group. When the list length is > 1 the returned metric
// will also contain statistics for mean/median/p50/p75/p90/p99 and sttdev.
func (p metricsList) aggregate() *CallMetrics {
	cm := &CallMetrics{
		FnName:      p[0].FnName,
		NestedCalls: make([]*CallMetrics, 0),

		MinTime:     time.Duration(math.MaxInt64),
		MaxTime:     time.Duration(math.MinInt64),
		Invocations: len(p),
	}

	// Pre-sort metrics so we can calculate the percentiles and the median
	if cm.Invocations > 0 {
		sort.Sort(p)

		// Calc min/max/total and pXX values
		callCountF := float64(len(p))
		p50 := int(math.Ceil(callCountF*.5)) - 1
		p75 := int(math.Ceil(callCountF*.75)) - 1
		p90 := int(math.Ceil(callCountF*.90)) - 1
		p99 := int(math.Ceil(callCountF*.99)) - 1

		cm.MinTime = p[0].TotalTime
		cm.MaxTime = p[len(p)-1].TotalTime
		cm.P50Time = p[p50].TotalTime
		cm.P75Time = p[p75].TotalTime
		cm.P90Time = p[p90].TotalTime
		cm.P99Time = p[p99].TotalTime

		for _, metric := range p {
			cm.TotalTime += metric.TotalTime
		}
	}

	// Calc mean
	cm.MeanTime = cm.TotalTime / time.Duration(cm.Invocations)

	// Calc stddev = Sqrt( 1 / N * Sum_i( (total_i - mean)^2 ) )
	for _, metric := range p {
		cm.StdDev += math.Pow(float64(metric.TotalTime-cm.MeanTime), 2.0)
	}
	cm.StdDev = math.Sqrt(cm.StdDev / float64(cm.Invocations))

	// Calc median
	if cm.Invocations%2 == 0 {
		cm.MedianTime = (p[(cm.Invocations/2-1)].TotalTime + p[cm.Invocations/2].TotalTime) / 2
	} else {
		cm.MedianTime = p[cm.Invocations/2].TotalTime
	}

	return cm
}

// CallMetrics encapsulates all collected metrics about a function call that is
// reachable by a profile target.
type CallMetrics struct {
	FnName string `json:"fn"`

	// Total time spent in this call.
	TotalTime time.Duration `json:"total_time"`

	// Min and max time.
	MinTime time.Duration `json:"min_time"`
	MaxTime time.Duration `json:"max_time"`

	// Mean and median time.
	MeanTime   time.Duration `json:"mean_time"`
	MedianTime time.Duration `json:"median_time"`

	// Percentiles.
	P50Time time.Duration `json:"p50_time"`
	P75Time time.Duration `json:"p75_time"`
	P90Time time.Duration `json:"p90_time"`
	P99Time time.Duration `json:"p99_time"`

	// Std of time valus.
	StdDev float64 `json:"std_dev"`

	// The number of times a scope was entered by the same parent function call.
	Invocations int `json:"invocations"`

	NestedCalls []*CallMetrics `json:"calls"`
}

type fnCall struct {
	fnName string

	// Time of entry/exit for this call.
	enteredAt time.Time
	exitedAt  time.Time

	// The overhead introduced by the profiler hooks.
	profilerOverhead time.Duration

	// The ordered list of calls originating from this call's scope.
	nestedCalls []*fnCall

	// The call via which this call was reached.
	parent *fnCall

	// The call group index this call belongs to. This field is populated
	// by the aggregateMetrics() call.
	callGroupIndex int
}

var callPool = sync.Pool{
	New: func() interface{} {
		return &fnCall{}
	},
}

// Allocate and initialize a new fnCall.
func makeFnCall(fnName string) *fnCall {
	call := callPool.Get().(*fnCall)
	call.fnName = fnName
	call.profilerOverhead = 0
	call.nestedCalls = make([]*fnCall, 0)
	call.parent = nil

	return call
}

// Perform a DFS on the fnCall tree releasing fnCall entries back to the call pool.
func (fn *fnCall) free() {
	for _, child := range fn.nestedCalls {
		child.free()
	}

	fn.nestedCalls = nil
	callPool.Put(fn)
}

// Append a fnCall instance to the set of nested calls.
func (fn *fnCall) nestCall(call *fnCall) {
	call.parent = fn
	fn.nestedCalls = append(fn.nestedCalls, call)
}

type callGroup struct {
	calls        []*fnCall
	nestedGroups []*callGroup
}

// callGroups groups fnCall sibling nodes at a particular fnCall tree depth by
// the composite key (fnCall.fnName, fnCall.parent.fnName)
type callGroups struct {
	nameToGroupIndex map[string]int
	groups           []*callGroup
}

// callGroupTree allows us to group together fnCall nodes belonging to similar
// execution paths so we can properly extract metrics for function scopes with
// multiple invocations. For more details see the docs on makeCallGroupTree()
type callGroupTree struct {
	levels []*callGroups
}

// aggregateMetrics takes as input a fnCall tree and emits a tree of aggregated
// CallMetrics. To illustrate how this works lets examine the following scenario:
//
// fnCall tree depth:
// 0  1   2
// --------
//
//        C1
//    B1--|
//    |   D1
// A--|
//    |   C2
//    B2--|
//        E1
//
// In this captured profile, A calls B twice (e.g. using a for loop) and B
// randomly calls 2 functions of the set [C, D, E]. Due to the way we capture
// our profiling data, each node in the graph is only aware of its direct
// children so the A->B2 branch is not aware of the A->B1 branch. In this scenario,
// C is actually invoked twice via B so we need to group these two paths together
// so we can calculate min/max/pXX stats for C.
//
// This function transforms the initial call tree using a 3-pass algorithm. The
// first pass performs a DFS on the tree grouping together similar (= with same name)
// sibling nodes at each level that are reached by a similar (= with same name)
// parent.
//
// The second pass performs a DFS on the output of the first pass
// nesting groups on level i under a i-1 level group when a i_th level group
// node has a parent belonging to the i-1_th group.
//
// After running the 2nd pass of the algorithm, we end up with a tree that looks
// like ([] denotes a list of grouped nodes):
//
//                   |- [C1, C2]
// [A] -|- [B1, B2] -|- [D1]
//                   |- [E1]
//
// The 3rd pass performs another DFS on the output of the second pass emitting
// an aggregated CallMetric for each set of grouped nodes yielding the final
// output (each item now being a CallMetric instance):
//          |- C
// A -|- B -|- D
//          |- E
func aggregateMetrics(rootFnCall *fnCall) *CallMetrics {
	cgt := &callGroupTree{
		levels: make([]*callGroups, 0),
	}

	cgt.insert(0, rootFnCall)
	cgt.linkGroups()

	// Run a DFS and group metrics starting at the top-level group which
	// only contains the rootFnCall node
	return cgt.groupMetrics(cgt.levels[0].groups[0])
}

// insert will recursively insert a call node and its children to the callGroup
// tree. At each tree level, nodes will be grouped with their sibling nodes using
// the composite key (fnCall.fnName, fnCall.parent.fnName)
func (t *callGroupTree) insert(depth int, call *fnCall) {
	if len(t.levels) < depth+1 {
		t.levels = append(t.levels, &callGroups{
			nameToGroupIndex: make(map[string]int, 0),
			groups:           make([]*callGroup, 0),
		})
	}

	level := t.levels[depth]

	// Construct composite grouping key
	var groupKey string
	if call.parent != nil {
		groupKey = call.parent.fnName + ","
	}
	groupKey += call.fnName

	groupIndex, exists := level.nameToGroupIndex[groupKey]
	if !exists {
		groupIndex = len(level.groups)
		level.groups = append(level.groups, &callGroup{
			calls:        make([]*fnCall, 0),
			nestedGroups: make([]*callGroup, 0),
		})
		level.nameToGroupIndex[groupKey] = groupIndex
	}

	call.callGroupIndex = groupIndex
	level.groups[groupIndex].calls = append(level.groups[groupIndex].calls, call)

	// Recursively process nested calls
	for _, nestedCall := range call.nestedCalls {
		t.insert(depth+1, nestedCall)
	}
}

// linkGroups connects groups at level i with groups at level i+1 when a
// direct path exists between the nodes in the two groups.
func (t *callGroupTree) linkGroups() {
	numLevels := len(t.levels)
	for levelNum := numLevels - 2; levelNum >= 0; levelNum-- {
		for levelGroupIndex, levelGroup := range t.levels[levelNum].groups {
			for _, subLevelGroup := range t.levels[levelNum+1].groups {
				for _, call := range subLevelGroup.calls {
					// A direct path exists from a node in levelGroup to
					// a node in subLevelGroup; link them together
					if call.parent.callGroupIndex == levelGroupIndex {
						levelGroup.nestedGroups = append(levelGroup.nestedGroups, subLevelGroup)
						break
					}
				}
			}
		}
	}
}

// groupMetrics generates a CallMetrics instance summarizing the individual
// CallMetrics for each fnCall instance in the given callGroup.
func (t *callGroupTree) groupMetrics(cg *callGroup) *CallMetrics {
	groupCallMetrics := make(metricsList, len(cg.calls))
	for callIndex, call := range cg.calls {
		groupCallMetrics[callIndex] = &CallMetrics{
			FnName:    call.fnName,
			TotalTime: call.exitedAt.Sub(call.enteredAt) - call.profilerOverhead,
		}
	}
	cm := groupCallMetrics.aggregate()

	// Iterate nested groups and append one aggregated metric per group
	for _, nestedGroup := range cg.nestedGroups {
		groupMetric := t.groupMetrics(nestedGroup)
		cm.NestedCalls = append(cm.NestedCalls, groupMetric)
	}

	return cm
}

// genProfile post-processes the data captured by the profiler into a Profile
// instance consisting of a tree structure of CallMetrics instances.
func genProfile(ID uint64, label string, rootFnCall *fnCall) *Profile {
	return &Profile{
		ID:        ID,
		CreatedAt: rootFnCall.enteredAt,
		Label:     label,
		Target:    aggregateMetrics(rootFnCall),
	}
}
