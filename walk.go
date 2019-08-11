package main

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/xerrors"
)

// pathedReader represents a reader that has an associated path
type pathedReader struct {
	path   string
	reader io.Reader
}

type pathWalker interface {
	// Walk takes a path and a function to process the file as an io.Reader.
	Walk(root string, process func(reader pathedReader) error) error
}

// fileWalker will only walk regular files
type fileWalker struct{}

// Walk acts as a simple wrapper for filepath.Walk, only processing regular files
func (walker fileWalker) Walk(path string, process func(reader pathedReader) error) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		// If we don't have a regular file, continue
		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(walkedPath)
		if err != nil {
			return xerrors.Errorf("could not open walked file (%s): %w", walkedPath, err)
		}

		defer file.Close()
		return process(pathedReader{path: walkedPath, reader: file})
	})
}
