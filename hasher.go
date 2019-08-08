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

// hashFileDetails represents all info about a file needed to hash it
type hashFileDetails struct {
	path string
	info os.FileInfo
}

// HashWalker represents something that can walk a tree and generate hashes
type HashWalker interface {
	// HashWalk takes a root path and returns a path of each file, along with its hash
	HashWalk(root string) map[string]hash.Hash
}

// SerialHashWalker will hash all files one after the other
// Implements HashWalker
type SerialHashWalker struct {
	constructor func() hash.Hash
}

// makehashForFile generates a hash for the given file, provided that it is a regular file.
// Returns errNotFile if the path is not a regular file
func hashFile(h hash.Hash, details hashFileDetails) error {
	if !details.info.Mode().IsRegular() {
		return errNotFile
	}

	fileHandle, err := os.Open(details.path)
	if err != nil {
		return xerrors.Errorf("failed to open file (%s) to hash: %w", details.path, err)
	}

	_, err = io.Copy(h, fileHandle)
	if err != nil {
		return xerrors.Errorf("could not hash file (%s): %w", details.path, err)
	}

	return nil
}

// NewSerialHashWalker makes a new serial hasher with a constructor for a hash algorithm
func NewSerialHashWalker(constructor func() hash.Hash) SerialHashWalker {
	return SerialHashWalker{
		constructor: constructor,
	}
}

// HashWalk walks the given path and returns hashes for all the files in the path
func (walker *SerialHashWalker) HashWalk(root string) (map[string]hash.Hash, error) {
	walkedMap := make(map[string]hash.Hash)
	// Walk all of the files and collect hashes for them
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}

		outHash := walker.constructor()
		details := hashFileDetails{path: path, info: info}
		err = hashFile(outHash, details)
		if err == errNotFile {
			// If we don't have a file, continue to the next one.
			return nil
		} else if err != nil {
			return xerrors.Errorf("could not process walked file: %w", err)
		}

		walkedMap[path] = outHash
		return nil
	})

	if err != nil {
		return nil, xerrors.Errorf("could not perform serial hash walk: %w", err)
	}

	return walkedMap, nil
}
