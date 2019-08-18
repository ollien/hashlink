package hashlink

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerialWalkHasher_HashWalk(t *testing.T) {
	testWalkHasherInterface(t, func(walker pathWalker, hashConstructor func() hash.Hash) WalkHasher {
		return makeSerialHashWalker(walker, hashConstructor)
	})
}

func TestParallelWalkhasher_HashWalk(t *testing.T) {
	testWalkHasherInterface(t, func(walker pathWalker, hashConstructor func() hash.Hash) WalkHasher {
		return makeParallelHashWalker(2, walker, hashConstructor)
	})
}

func testWalkHasherInterface(t *testing.T, makeHasher func(walker pathWalker, hashConstructor func() hash.Hash) WalkHasher) {
	files := map[string]string{
		"a/b":    "hello world",
		"a/bb/c": "my awesome file!",
		"a/bb/d": "unit testing...",
		"a/bb/e": "this is the last file I'm testing",
	}

	hashes := map[string]string{
		"a/b":    "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		"a/bb/c": "6cd8ca076b44600d0c183520c0c30bd6d65995b11a36727dcee777fa8e6f5ad0",
		"a/bb/d": "100182cad7531dc4c202e34ee5c666ea284c66196f1bfee24812d11ba1543d86",
		"a/bb/e": "d6f542548b05eeef1e909a850dd3f3e383caffdb7e59f059b739584322fee77f",
	}
	expectedFiles := []string{"a/b", "a/bb/c", "a/bb/d", "a/bb/e"}

	// staticWalker defined in walk_test.go
	walker := staticWalker{files: files, readers: make(map[string]*closableStringReader, len(files))}
	hasher := makeHasher(walker, sha256.New)
	walkedHashes, err := hasher.WalkAndHash("a")
	assert.Nil(t, err)

	hashBuffer := make([]byte, 0)
	hashedFiles := make([]string, 0, len(walkedHashes))
	for path, hash := range walkedHashes {
		hashedFiles = append(hashedFiles, path)
		sum := hash.Sum(hashBuffer)
		// Assert that the hash matches for the given path
		assert.Equal(t, hashes[path], hex.EncodeToString(sum))
	}

	assert.ElementsMatch(t, expectedFiles, hashedFiles)
	// Ensure that we have made ar eader for every single file
	assert.Equal(t, len(files), len(walker.readers))
	// Assert that every file has been closed exactly once.
	for filename, reader := range walker.readers {
		assert.Equal(t, 1, reader.closeCount, "file="+filename)
	}
}
