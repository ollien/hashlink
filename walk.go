package hashlink

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/xerrors"
)

// pathedData represents a some kind of data that has an associated filesystem path
type pathedData struct {
	path string
	data io.ReadCloser
}

type pathWalker interface {
	// Walk takes a path and a function to process the file as an io.Reader.
	Walk(root string, process func(reader pathedData) error) error
}

// fileWalker will only walk regular files
type fileWalker struct{}

// open will open the data at the path if needed.
func (data pathedData) open() (io.ReadCloser, error) {
	// If we've already opened the file, don't re-open it
	if data.data != nil {
		return data.data, nil
	}

	openedFile, err := os.Open(data.path)
	if err != nil {
		err = xerrors.Errorf("could not open file (%s): %w", data.path, err)
	}

	data.data = openedFile

	return openedFile, nil
}

// Walk acts as a simple wrapper for filepath.Walk, only processing regular files
func (walker fileWalker) Walk(path string, process func(reader pathedData) error) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return xerrors.Errorf("could not walk: %w", err)
		}

		// If we don't have a regular file, continue
		if !info.Mode().IsRegular() {
			return nil
		}

		return process(pathedData{path: walkedPath})
	})
}

func getAllItemsFromWalker(walker pathWalker, path string) ([]pathedData, error) {
	result := make([]pathedData, 0)
	err := walker.Walk(path, func(reader pathedData) error {
		result = append(result, reader)

		return nil
	})

	return result, err
}
