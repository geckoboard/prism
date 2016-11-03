package sink

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geckoboard/prism/profiler"
)

func TestFileSink(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "prism-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s := NewFileSink(tmpDir)
	err = s.Open(0)
	if err != nil {
		t.Fatal(err)
	}

	numEntries := 10

	var wg sync.WaitGroup
	wg.Add(numEntries)
	for i := 0; i < numEntries; i++ {
		go func(i int) {
			defer wg.Done()
			// This gets incorrectly flagged as a data-race but
			// in reality no race exists as we are sending data to the channel
			s.Input() <- &profiler.Entry{
				Name: fmt.Sprintf(`foo.bar/baz\boo.%d`, i),
			}
		}(i)
	}

	wg.Wait()

	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	fileList, err := filepath.Glob(tmpDir + "/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(fileList) != numEntries {
		t.Errorf("expected number of written files to be %d; got %d", numEntries, len(fileList))
	}

	for _, fpath := range fileList {
		fname := strings.TrimSuffix(fpath[len(tmpDir)+1:], ".json")
		if !strings.HasPrefix(fname, profilePrefix) {
			t.Errorf("[%s] expected prefix to be %q", fname, profilePrefix)
		}

		badCharIndex := strings.IndexAny(fname, `./\`)
		if badCharIndex != -1 {
			t.Errorf("[%s] found invalid char %q at index %d", fname, fname[badCharIndex:badCharIndex+1], badCharIndex)
		}
	}
}
