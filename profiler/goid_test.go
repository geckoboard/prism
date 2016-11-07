package profiler

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestThreadID(t *testing.T) {
	var buf = make([]byte, 64)
	runtime.Stack(buf, false)
	tokens := strings.Split(string(buf), " ")
	expID, err := strconv.ParseUint(tokens[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	tid := threadID()

	if tid != expID {
		t.Fatalf("expected threadID() to return %d; got %d", expID, tid)
	}
}

func TestParseBase10UintBytes(t *testing.T) {
	specs := []struct {
		Bits   int
		Input  []byte
		ExpVal uint64
		ExpErr error
	}{
		{64, []byte{'1', '2', '3', '4', '5'}, 12345, nil},
		{64, []byte{'A', '2', '3', '4', '5'}, 0, strconv.ErrSyntax},
		{64, []byte{}, 0, strconv.ErrSyntax},
		{64, []byte{'9', '6', '7', '6', '9', '7', '6', '7', '3', '3', '9', '7', '3', '5', '9', '5', '6', '0', '0', '0'}, 1<<64 - 1, strconv.ErrRange},
		{32, []byte{'9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '6', '7', '6', '9', '7', '6', '7', '3', '3', '9', '7', '3', '5', '9', '5', '6', '0', '0', '0'}, 1<<64 - 1, strconv.ErrRange},
	}

	for specIndex, spec := range specs {
		val, err := parseBase10UintBytes(spec.Input, spec.Bits)
		if spec.ExpErr == nil && err != nil {
			t.Errorf("[spec %d] error parsing value: %v", specIndex, err)
		}

		if spec.ExpErr != nil && (err == nil || !strings.Contains(err.Error(), spec.ExpErr.Error())) {
			t.Errorf("[spec %d] expected parse error %q; got %v", specIndex, spec.ExpErr.Error(), err)
		}

		if val != spec.ExpVal {
			t.Errorf("[spec %d] expected parsed value to be %d; got %d", specIndex, spec.ExpVal, val)
		}
	}
}
