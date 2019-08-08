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

// SerialWalkHasher will hash all files one after the other
// Implements HashWalker
type SerialWalkHasher struct {
	constructor func() hash.Hash
	walker      pathWalker
}

// NewSerialWalkHasher makes a new serial hasher with a constructor for a hash algorithm
func NewSerialWalkHasher(constructor func() hash.Hash) SerialWalkHasher {
	walker := fileWalker{}

	return makeSerialHashWalker(walker, constructor)
}

// makeSerialHashWalker will build a serial hash walker with the given spec. Used mainly as faux-dependency injection
func makeSerialHashWalker(walker pathWalker, constructor func() hash.Hash) SerialWalkHasher {
	return SerialWalkHasher{
		walker:      walker,
		constructor: constructor,
	}
}

// WalkAndHash walks the given path and returns hashes for all the files in the path
func (hasher SerialWalkHasher) WalkAndHash(root string) (map[string]hash.Hash, error) {
	walkedMap := make(map[string]hash.Hash)
	// Walk all of the files and collect hashes for them
	err := hasher.walker.Walk(root, func(path string, reader io.Reader) error {
		outHash := hasher.constructor()
		_, err := io.Copy(outHash, reader)
		if err != nil {
			return xerrors.Errorf("could not hash walked file (%s): %w", path, err)
		}

		walkedMap[path] = outHash
		return nil
	})

	if err != nil {
		return nil, xerrors.Errorf("could not perform serial hash walk: %w", err)
	}

	return walkedMap, nil
}
