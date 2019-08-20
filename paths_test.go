package hashlink

/*
	Copyright 2019 Nicholas Krichevsky

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pathTest struct {
	name string
	test func(t *testing.T)
}

func runPathTestTable(t *testing.T, table []pathTest) {
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}

func TestFindIdenticalFiles(t *testing.T) {
	tests := []pathTest{
		pathTest{
			name: "empty maps",
			test: func(t *testing.T) {
				hashes := PathHashes{}
				otherHashes := PathHashes{}
				res := FindIdenticalFiles(hashes, otherHashes)
				assert.Equal(t, FileMap{}, res)
			},
		},
		pathTest{
			name: "empty source map",
			test: func(t *testing.T) {
				hashes := PathHashes{}
				hash := sha256.New()
				hash.Write([]byte("blah"))
				otherHashes := PathHashes{"a/b": hash}
				res := FindIdenticalFiles(hashes, otherHashes)
				assert.Equal(t, FileMap{}, res)
			},
		},
		pathTest{
			name: "one matching hash",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("oh no"))
				hash2 := sha256.New()
				hash2.Write([]byte("oh yes"))
				hashes := PathHashes{
					"a/b": hash1,
					"a/c": hash2,
				}

				otherHash1 := sha256.New()
				otherHash1.Write([]byte("oh no"))
				otherHash2 := sha256.New()
				otherHash2.Write([]byte("nah"))
				otherHashes := PathHashes{
					"b/b": otherHash1,
					"c/c": otherHash2,
				}

				res := FindIdenticalFiles(hashes, otherHashes)
				assert.Equal(t, FileMap{
					"a/b": []string{"b/b"},
				}, res)
			},
		},
		pathTest{
			name: "duplicate file in source",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("oh no"))
				hash2 := sha256.New()
				hash2.Write([]byte("oh yes"))
				hashes := PathHashes{
					"a/b": hash1,
					"a/c": hash1,
				}

				otherHash1 := sha256.New()
				otherHash1.Write([]byte("oh no"))
				otherHash2 := sha256.New()
				otherHash2.Write([]byte("nah"))
				otherHashes := PathHashes{
					"b/b": otherHash1,
					"c/c": otherHash2,
				}

				res := FindIdenticalFiles(hashes, otherHashes)
				assert.Equal(t, FileMap{
					"a/b": []string{"b/b"},
					"a/c": []string{"b/b"},
				}, res)
			},
		},
		pathTest{
			name: "duplicate file in other",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("oh no"))
				hash2 := sha256.New()
				hash2.Write([]byte("oh yes"))
				hashes := PathHashes{
					"a/b": hash1,
					"a/c": hash2,
				}

				otherHash1 := sha256.New()
				otherHash1.Write([]byte("oh no"))
				otherHash2 := sha256.New()
				otherHash2.Write([]byte("nah"))
				otherHashes := PathHashes{
					"b/b": otherHash1,
					"c/c": otherHash1,
				}

				res := FindIdenticalFiles(hashes, otherHashes)
				// Because this test can return multiple values, we need to use ElementsMatch on each individual slice.
				expected := FileMap{
					"a/b": []string{"b/b", "c/c"},
				}

				for key, value := range res {
					assert.Contains(t, expected, key)
					assert.ElementsMatch(t, expected[key], value)
				}
			},
		},
	}
	runPathTestTable(t, tests)
}

func TestGetUnmappedFiles(t *testing.T) {
	tests := []pathTest{
		pathTest{
			name: "no files",
			test: func(t *testing.T) {
				hashes := PathHashes{}
				files := FileMap{}
				unmappedFiles := GetUnmappedFiles(hashes, files)
				assert.Equal(t, []string{}, unmappedFiles)
			},
		},
		pathTest{
			name: "only hash files",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("pls"))
				hash2 := sha256.New()
				hash2.Write([]byte("I don't matter"))
				hashes := PathHashes{
					"a/b": hash1,
					"b/c": hash2,
				}

				files := FileMap{}
				unmappedFiles := GetUnmappedFiles(hashes, files)
				assert.ElementsMatch(t, []string{"a/b", "b/c"}, unmappedFiles)
			},
		},
		pathTest{
			name: "only mapped files",
			test: func(t *testing.T) {
				hashes := PathHashes{}
				files := FileMap{
					"a/b": []string{"something"},
					"b/c": []string{"somethingelse"},
				}

				unmappedFiles := GetUnmappedFiles(hashes, files)
				assert.ElementsMatch(t, []string{}, unmappedFiles)
			},
		},
		pathTest{
			name: "full intersection",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("pls"))
				hash2 := sha256.New()
				hash2.Write([]byte("I don't matter"))
				hashes := PathHashes{
					"a/b": hash1,
					"b/c": hash2,
				}
				files := FileMap{
					"a/b": []string{"something"},
					"b/c": []string{"somethingelse"},
				}

				unmappedFiles := GetUnmappedFiles(hashes, files)
				assert.ElementsMatch(t, []string{}, unmappedFiles)
			},
		},
		pathTest{
			name: "partial intersection",
			test: func(t *testing.T) {
				hash1 := sha256.New()
				hash1.Write([]byte("pls"))
				hash2 := sha256.New()
				hash2.Write([]byte("I don't matter"))
				hashes := PathHashes{
					"a/b": hash1,
					"b/c": hash2,
				}
				files := FileMap{
					"a/b": []string{"something"},
				}

				unmappedFiles := GetUnmappedFiles(hashes, files)
				assert.ElementsMatch(t, []string{"b/c"}, unmappedFiles)
			},
		},
	}

	runPathTestTable(t, tests)
}

func TestMakeFlippedFileMap(t *testing.T) {
	tests := []pathTest{
		pathTest{
			name: "no files",
			test: func(t *testing.T) {
				files := FileMap{}
				flipped := MakeFlippedFileMap(files)
				assert.Equal(t, FileMap{}, flipped)
			},
		},
		pathTest{
			name: "unique files",
			test: func(t *testing.T) {
				files := FileMap{
					"a/b": []string{"b/c"},
					"d/e": []string{"f/g", "h/i"},
				}
				flipped := MakeFlippedFileMap(files)
				expected := FileMap{
					"b/c": []string{"a/b"},
					"f/g": []string{"d/e"},
					"h/i": []string{"d/e"},
				}

				for key := range expected {
					assert.ElementsMatch(t, expected[key], flipped[key])
				}
			},
		},
		pathTest{
			name: "non-unique files",
			test: func(t *testing.T) {
				files := FileMap{
					"a/b": []string{"b/c"},
					"d/e": []string{"b/c", "g/h"},
				}

				flipped := MakeFlippedFileMap(files)
				expected := FileMap{
					"b/c": []string{"a/b", "d/e"},
					"g/h": []string{"d/e"},
				}

				for key := range expected {
					assert.Contains(t, flipped, key)
					assert.ElementsMatch(t, expected[key], flipped[key])
				}
			},
		},
	}

	runPathTestTable(t, tests)
}
