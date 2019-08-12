package hashlink

import (
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"sync"

	"golang.org/x/xerrors"
)

var errNotFile = errors.New("not a file; will not hash")

// PathHashes represent the hashes for all paths walked by a WalkHasher, with the path as the key,
// and the hash as the value.
type PathHashes map[string]hash.Hash

// WalkHasher represents something that can walk a tree and generate hashes
type WalkHasher interface {
	// WalkAndHash takes a root path and returns a path of each file, along with its hash
	WalkAndHash(root string) (PathHashes, error)
}

// hashReader will hash a reader into the given hash interface, h
func hashReader(h hash.Hash, reader io.Reader) (retErr error) {
	_, err := io.Copy(h, reader)
	if err != nil {
		retErr = xerrors.Errorf("could not hash file: %w", err)
	}

	return
}

// convertSyncMapToResultMap takes a syncMap and converts it to a result of parallel
func makePathHashesFromSyncMap(syncMap *sync.Map) PathHashes {
	resultMap := make(PathHashes)
	syncMap.Range(func(key, value interface{}) bool {
		// We should panic if these values aren't the correct type - there is no way we can produce a result that is correct
		keyStr := key.(string)
		valueHash := value.(hash.Hash)
		resultMap[keyStr] = valueHash

		return true
	})

	return resultMap
}

// MapIdenticalPaths will return a map of paths within it to paths within the other, based on the equality of their hashes
func (hashes PathHashes) MapIdenticalPaths(other PathHashes) map[string][]string {
	flipped := hashes.flip()
	flippedOther := other.flip()
	res := make(map[string][]string)
	for hash, paths := range flipped {
		otherPaths, havePaths := flippedOther[hash]
		if !havePaths {
			continue
		}

		for _, path := range paths {
			res[path] = append(res[path], otherPaths...)
		}
	}

	return res
}

// flip will flip the map, and bucket all non-unique hashes into one key, where the keys are string digests of the hash
// hash.Hashes are not compariable on their own, thus we need to encode them.
func (hashes PathHashes) flip() map[string][]string {
	res := make(map[string][]string)
	sum := make([]byte, 0)
	for path, hash := range hashes {
		sum = hash.Sum(sum)
		key := hex.EncodeToString(sum)
		sum = sum[:0]
		res[key] = append(res[key], path)
	}

	return res
}
