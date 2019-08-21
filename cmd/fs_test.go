package main

import (
	"os"
	"testing"

	"github.com/ollien/hashlink"
	"github.com/stretchr/testify/assert"
)

// opArgs represents a set of arguments given to an op function
type opArgs struct {
	src, dst string
}

// mockOpWrapper is a wrapper for a connectFunction that will record all calls given to it
type mockOpWrapper struct {
	calls []opArgs
}

// op is a bsic connectFunction that will record what calls are given to it, and return no error
func (m *mockOpWrapper) op(src, dst string) error {
	m.calls = append(m.calls, opArgs{src, dst})

	return nil
}

type connectTest struct {
	name string
	test func(t *testing.T, opWrapper mockOpWrapper)
}

func runConnectTestTable(t *testing.T, table []connectTest) {
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t, mockOpWrapper{})
		})
	}
}

func TestConnectMappedFiles(t *testing.T) {
	tests := []connectTest{
		{
			name: "no files",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				err := connectMappedFiles(hashlink.FileMap{}, "foo/ref", "foo/out", opWrapper.op)
				assert.Nil(t, err)
				assert.ElementsMatch(t, []opArgs{}, opWrapper.calls)
			},
		},
		{
			name: "some files",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				files := hashlink.FileMap{
					"src/b":           []string{"foo/ref/b"},
					"src/a":           []string{"foo/ref/c"},
					"src/d":           []string{"foo/ref/e", "foo/ref/dir/f"},
					"src/something/g": []string{"foo/ref/g"},
				}

				err := connectMappedFiles(files, "foo/ref", "foo/out", opWrapper.op)
				assert.Nil(t, err)
				assert.ElementsMatch(t, []opArgs{
					{src: "src/b", dst: "foo/out/b"},
					{src: "src/a", dst: "foo/out/c"},
					{src: "src/d", dst: "foo/out/e"},
					{src: "src/d", dst: "foo/out/dir/f"},
					{src: "src/something/g", dst: "foo/out/g"},
				}, opWrapper.calls)
			},
		},
		{
			name: "file not relative to reference dir",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				files := hashlink.FileMap{
					"src/b": []string{"foo/ref/b"},
					"src/a": []string{"/wrong/location"},
					"src/c": []string{"foo/ref/d"},
				}

				err := connectMappedFiles(files, "foo/ref", "foo/out", opWrapper.op)
				assert.NotNil(t, err)
				// We should still call op for all of the elements we can
				assert.ElementsMatch(t, []opArgs{
					{src: "src/b", dst: "foo/out/b"},
					{src: "src/c", dst: "foo/out/d"},
				}, opWrapper.calls)
			},
		},
	}

	runConnectTestTable(t, tests)
}

func TestConnectFiles(t *testing.T) {
	tests := []connectTest{
		{
			name: "no files",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				err := connectFiles([]string{}, "foo/ref", "foo/out", opWrapper.op)
				assert.Nil(t, err)
				assert.ElementsMatch(t, []opArgs{}, opWrapper.calls)
			},
		},
		{
			name: "some files",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				files := []string{
					"foo/ref/dir/a_file",
					"foo/ref/another_file",
				}

				err := connectFiles(files, "foo/ref", "foo/out", opWrapper.op)
				assert.Nil(t, err)
				assert.ElementsMatch(t, []opArgs{
					{src: "foo/ref/dir/a_file", dst: "foo/out/dir/a_file"},
					{src: "foo/ref/another_file", dst: "foo/out/another_file"},
				}, opWrapper.calls)
			},
		},
		{
			name: "file not relative to reference dir",
			test: func(t *testing.T, opWrapper mockOpWrapper) {
				files := []string{
					"foo/ref/dir/a_file",
					"foo/ref/another_file",
					"/wrong/location",
				}

				err := connectFiles(files, "foo/ref", "foo/out", opWrapper.op)
				assert.NotNil(t, err)
				// We should still call op for all of the elements we can
				assert.ElementsMatch(t, []opArgs{
					{src: "foo/ref/dir/a_file", dst: "foo/out/dir/a_file"},
					{src: "foo/ref/another_file", dst: "foo/out/another_file"},
				}, opWrapper.calls)
			},
		},
	}

	runConnectTestTable(t, tests)
}

type fsTest struct {
	name string
	test func(t *testing.T)
}

func runFsTestTable(t *testing.T, table []fsTest) {
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}

func TestRemoveExecuteBits(t *testing.T) {
	tests := []fsTest{
		{
			name: "remove execute bits",
			test: func(t *testing.T) {
				assert.Equal(t, os.FileMode(0644), removeExecuteBits(os.FileMode(0755)))
			},
		},
		{
			name: "don't change the mode if we have something non-executable",
			test: func(t *testing.T) {
				assert.Equal(t, os.FileMode(0644), removeExecuteBits(os.FileMode(0644)))
			},
		},
		{
			name: "keep bits that are already non-executable",
			test: func(t *testing.T) {
				assert.Equal(t, os.FileMode(0600), removeExecuteBits(os.FileMode(0611)))
			},
		},
	}

	runFsTestTable(t, tests)
}
