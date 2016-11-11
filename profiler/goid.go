package profiler

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
)

const (
	base10CutOff = (1<<64 - 1) / 11
)

var (
	goRoutinePrefix = []byte("goroutine ")
)

// Implementation copied by https://github.com/tylerb/gls/blob/2ef09cd25215bcab07d95380475175ba9a9fdc40/gotrack.go
var stackBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64)
		return &buf
	},
}

// Detect the current goroutine id.
func threadID() uint64 {
	bp := stackBufPool.Get().(*[]byte)
	defer stackBufPool.Put(bp)
	b := *bp
	b = b[:runtime.Stack(b, false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goRoutinePrefix)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		panic("threadID(): [BUG] missing space at goRoutinePrefix")
	}
	b = b[:i]
	n, err := parseBase10UintBytes(b, 64)
	if err != nil {
		panic("threadID(): [BUG] failed to parse goroutine ID")
	}
	return n
}

// parseUintBytes works like strconv.ParseUint with base=10, but using a []byte.
func parseBase10UintBytes(s []byte, bitSize int) (n uint64, err error) {
	if len(s) == 0 {
		err = strconv.ErrSyntax
		return n, &strconv.NumError{Func: "ParseUint", Num: string(s), Err: err}
	}

	n = 0
	var maxVal uint64 = 1<<uint(bitSize) - 1

	for i := 0; i < len(s); i++ {
		var v byte
		d := s[i]
		switch {
		case '0' <= d && d <= '9':
			v = d - '0'
		default:
			n = 0
			err = strconv.ErrSyntax
			return n, &strconv.NumError{Func: "parseBase10UintBytes", Num: string(s), Err: err}
		}

		if n >= base10CutOff {
			n = 1<<64 - 1
			err = strconv.ErrRange
			return n, &strconv.NumError{Func: "parseBase10UintBytes", Num: string(s), Err: err}
		}
		n *= 10

		n1 := n + uint64(v)
		if n1 < n || n1 > maxVal {
			// n+v overflows
			n = 1<<64 - 1
			err = strconv.ErrRange
			return n, &strconv.NumError{Func: "parseBase10UintBytes", Num: string(s), Err: err}
		}
		n = n1
	}

	return n, nil
}
