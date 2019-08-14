package hashlink

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
				assert.Equal(t, FileMap{
					"a/b": []string{"b/b", "c/c"},
				}, res)
			},
		},
	}
	runPathTestTable(t, tests)
}
