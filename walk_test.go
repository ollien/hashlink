package hashlink

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// closableStringReader serves as a wrapper for *strings.Reader to allow it to implement the io.ReadCloser interface
type closableStringReader struct {
	closeCount int
	*strings.Reader
}

// staticWalker is a mock walker for use with testing
type staticWalker struct {
	// A map of file names to file contents
	files map[string]string
	// A map of file names to closableStringReaders
	readers map[string]*closableStringReader
}

// Walk will simply return io.ReadClosers (within pathedData) of all of the files within the given root. Note that
// process must close the file once it is doneA.
func (walker staticWalker) Walk(root string, process func(reader pathedData) error) error {
	// Ignore the root - it doesn't matter for our case here.
	for filename, contents := range walker.files {
		reader := &closableStringReader{Reader: strings.NewReader(contents)}
		walker.readers[filename] = reader
		err := process(pathedData{path: filename, data: reader})
		if err != nil {
			return err
		}
	}

	return nil
}

// Close will simply nop. Implemented so strings.Reader can fufill the ReadCloser interface.
func (r *closableStringReader) Close() error {
	r.closeCount++

	return nil
}

type walkTest struct {
	name  string
	setup func() pathWalker
	test  func(t *testing.T, walker pathWalker)
}

func runWalkTestTable(t *testing.T, table []walkTest) {
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			walker := tt.setup()
			tt.test(t, walker)
		})
	}
}

func TestGetAllItemsFromwalker(t *testing.T) {
	tests := []walkTest{
		walkTest{
			name: "no files",
			setup: func() pathWalker {
				files := map[string]string{}

				return staticWalker{files: files, readers: make(map[string]*closableStringReader, len(files))}
			},
			test: func(t *testing.T, walker pathWalker) {
				result, err := getAllItemsFromWalker(walker, "/")
				assert.Nil(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, 0, len(result))
			},
		},
		walkTest{
			name: "bunch of files",
			setup: func() pathWalker {
				files := map[string]string{
					"a/b":    "hello world",
					"a/bb/c": "my awesome file!",
					"a/bb/d": "unit testing...",
					"a/bb/e": "this is the last file I'm testing",
				}

				return staticWalker{files: files, readers: make(map[string]*closableStringReader, len(files))}
			},
			test: func(t *testing.T, walker pathWalker) {
				result, err := getAllItemsFromWalker(walker, "/")
				assert.Nil(t, err)
				assert.ElementsMatch(t, []string{"a/b", "a/bb/c", "a/bb/d", "a/bb/e"}, result)
				// Assert that every file has been closed exactly once.
				for filename, reader := range walker.(staticWalker).readers {
					assert.Equal(t, 1, reader.closeCount, "file="+filename)
				}
			},
		},
	}

	runWalkTestTable(t, tests)
}