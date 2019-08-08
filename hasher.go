package main

import (
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/xerrors"
)

var errNotFile = errors.New("not a file; will not hash")

// HashWalker represents something that can walk a tree and generate hashes
type HashWalker interface {
	// HashWalk takes a root path and returns a path of each file, along with its hash
	HashWalk(root string) map[string]hash.Hash
}

// SerialHashWalker will hash all files one after the other
type SerialHashWalker struct {
	constructor func() hash.Hash
}

// NewSerialHashWalker makes a new serial hasher with a constructor for the new hash algorithm
// Implements HashWalker
func NewSerialHashWalker(constructor func() hash.Hash) SerialHashWalker {
	return SerialHashWalker{
		constructor: constructor,
	}
}

// HashWalk walks the given path and returns all
func (walker *SerialHashWalker) HashWalk(root string) (map[string]hash.Hash, error) {
	walkedMap := make(map[string]hash.Hash)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}

		hash, err := walker.makeHashForFile(path, info)
		if err == errNotFile {
			// If we don't have a file, continue to the next one.
			return nil
		} else if err != nil {
			return xerrors.Errorf("could not process walked file: %w", err)
		}

		walkedMap[path] = hash
		return nil
	})

	if err != nil {
		return nil, xerrors.Errorf("could not perform serial hash walk: %w", err)
	}

	return walkedMap, nil
}

// makehashForFile generates a hash for the given file, provided that it is a regular file.
// Returns errNotFile if the path is not a regular file
func (walker *SerialHashWalker) makeHashForFile(path string, info os.FileInfo) (hash.Hash, error) {
	if !info.Mode().IsRegular() {
		return nil, errNotFile
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, xerrors.Errorf("failed to open file (%s) to hash: %w", path, err)
	}

	hash := walker.constructor()
	_, err = io.Copy(hash, f)
	if err != nil {
		return nil, xerrors.Errorf("could not hash file (%s): %w", path, err)
	}

	return hash, nil
}
