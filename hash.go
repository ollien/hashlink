package hashlink

import (
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
