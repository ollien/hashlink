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
	"hash"

	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

// SerialWalkHasher will hash all files one after the other.
// Implements HashWalker.
type SerialWalkHasher struct {
	constructor      func() hash.Hash
	walker           pathWalker
	progressReporter ProgressReporter
}

// SerialWalkHasherProgressReporter will provide a ProgressReporter for a SerialWalkHasher.
// Intended to be passed to NewSerialWalkHasher as an option.
func SerialWalkHasherProgressReporter(reporter ProgressReporter) func(*SerialWalkHasher) {
	return func(hasher *SerialWalkHasher) {
		hasher.progressReporter = reporter
	}
}

// NewSerialWalkHasher makes a new SerialWalkHasher with a constructor for a hash algorithm.
func NewSerialWalkHasher(constructor func() hash.Hash, options ...func(*SerialWalkHasher)) *SerialWalkHasher {
	walker := fileWalker{}

	return makeSerialHashWalker(walker, constructor, options...)
}

// makeSerialHashWalker will build a SerialWalkHasher with the given spec. Used mainly as faux-dependency injection.
func makeSerialHashWalker(walker pathWalker, constructor func() hash.Hash, options ...func(*SerialWalkHasher)) *SerialWalkHasher {
	hasher := &SerialWalkHasher{
		walker:           walker,
		constructor:      constructor,
		progressReporter: nilProgressReporter{},
	}

	for _, optionFunc := range options {
		optionFunc(hasher)
	}

	return hasher
}

// WalkAndHash walks the given path and returns hashes for all the files in the path.
func (hasher SerialWalkHasher) WalkAndHash(root string) (PathHashes, error) {
	walkedMap := make(PathHashes)
	// Walk all of the files and collect hashes for them
	walkerItems, err := getAllItemsFromWalker(hasher.walker, root)
	if err != nil {
		return nil, xerrors.Errorf("could not get items for a serial hash walk: %w", err)
	}

	errors := multierror.NewMultiError()
	hasher.progressReporter.ReportProgress(Progress(0))
	for i, reader := range walkerItems {
		outHash, err := hasher.processData(reader)
		hasher.progressReporter.ReportProgress(Progress(i * 100 / len(walkerItems)))
		if err != nil {
			errors.Append(err)
			continue
		}

		walkedMap[reader.path] = outHash
	}

	if errors.Len() > 0 {
		return nil, xerrors.Errorf("could not perform serial hash walker: %w", errors)
	}

	return walkedMap, nil
}

// processData will perform the hash and any cleanup needed for the given reader.
func (hasher SerialWalkHasher) processData(reader pathedData) (hash.Hash, error) {
	data, err := reader.open()
	if err != nil {
		err = xerrors.Errorf("could not open data for path (%s)", reader.path, err)
		return nil, err
	}

	defer data.Close()
	outHash := hasher.constructor()
	err = hashReader(outHash, data)
	if err != nil {
		err = xerrors.Errorf("could not hash path (%s): %w", reader.path, err)
		return nil, err
	}

	return outHash, nil
}
