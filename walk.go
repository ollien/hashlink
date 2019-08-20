package hashlink

/*
	Copyright 2019 Nicholas Krichevsky

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

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
		return nil, err
	}

	data.data = openedFile

	return openedFile, nil
}

// Walk acts as a simple wrapper for filepath.Walk, only processing regular files.
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

// getAllItemsFromWalker gets every item that the given pathWalker would pass to its callback.
func getAllItemsFromWalker(walker pathWalker, path string) ([]pathedData, error) {
	result := make([]pathedData, 0)
	err := walker.Walk(path, func(reader pathedData) error {
		result = append(result, reader)

		return nil
	})

	return result, err
}
