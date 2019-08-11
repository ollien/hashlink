package main

import (
	"errors"
	"hash"
	"io"

	"golang.org/x/xerrors"
)

var errNotFile = errors.New("not a file; will not hash")

// WalkHasher represents something that can walk a tree and generate hashes
type WalkHasher interface {
	// WalkAndHash takes a root path and returns a path of each file, along with its hash
	WalkAndHash(root string) (map[string]hash.Hash, error)
}

// pathedHash represents a hash associated with a path
type pathedHash struct {
	path string
	hash hash.Hash
}

// hashReader will hash a reader into the given hash interface, h
func hashReader(h hash.Hash, reader io.Reader) (retErr error) {
	_, err := io.Copy(h, reader)
	if err != nil {
		retErr = xerrors.Errorf("could not hash file: %w", err)
	}

	return
}
