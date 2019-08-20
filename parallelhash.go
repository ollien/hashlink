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
	"context"
	"hash"
	"sync"

	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

// ParallelWalkHasher will hash all files concurrently, up to the number of specified workers.
type ParallelWalkHasher struct {
	constructor      func() hash.Hash
	walker           pathWalker
	numWorkers       int
	progressReporter ProgressReporter
}

// hashResult represents the result of a hashing operation.
type hashResult struct {
	// path represents the location that has been hashed.
	path string
	// hash represents the hash of the data located at path.
	hash hash.Hash
	// If an error occurred during operation, then err will be non-nil.
	err error
}

// ParallelWalkHasherProgressReporter will provide a ProgressReporter for a ParallelWalkWasher.
// Intended to be passed to NewParallelWalkHasher as an option.
func ParallelWalkHasherProgressReporter(reporter ProgressReporter) func(*ParallelWalkHasher) {
	return func(hasher *ParallelWalkHasher) {
		hasher.progressReporter = reporter
	}
}

// NewParallelWalkHasher makekes a new ParallelWalkHasher with a constructor for a hash algorithm and a number
// of workers.
func NewParallelWalkHasher(numWorkers int, constructor func() hash.Hash, options ...func(*ParallelWalkHasher)) *ParallelWalkHasher {
	walker := fileWalker{}

	return makeParallelHashWalker(numWorkers, walker, constructor, options...)
}

// makeParallelHashWalker will build a ParallelWalkHasher with the given spec. Used mainly as faux-dependency injection
func makeParallelHashWalker(numWorkers int, walker pathWalker, constructor func() hash.Hash, options ...func(*ParallelWalkHasher)) *ParallelWalkHasher {
	hasher := &ParallelWalkHasher{
		walker:           walker,
		constructor:      constructor,
		numWorkers:       numWorkers,
		progressReporter: nilProgressReporter{},
	}

	for _, optionFunc := range options {
		optionFunc(hasher)
	}

	return hasher
}

// WalkAndHash walks the given path across all workers and returns hashes for all the files in the path.
func (hasher *ParallelWalkHasher) WalkAndHash(root string) (PathHashes, error) {
	walkerItems, err := getAllItemsFromWalker(hasher.walker, root)
	if err != nil {
		return nil, xerrors.Errorf("could not perform get items for parallel hash walk: %w", err)
	}

	hasher.progressReporter.ReportProgress(Progress(0))
	ctx, cancelFunc := context.WithCancel(context.Background())
	workerWaitGroup := sync.WaitGroup{}
	workChan := make(chan pathedData)
	errorChan := make(chan error)

	// Spawn all workers, and send work to them
	resultChan := hasher.spawnWorkers(ctx, &workerWaitGroup, workChan)
	collectedResultChannel := hasher.collectResults(cancelFunc, resultChan, errorChan)
	collectedErrorChannel := hasher.collectErrors(errorChan)
	hasher.dispatchWork(ctx, walkerItems, workChan)

	close(workChan)
	workerWaitGroup.Wait()
	results := <-collectedResultChannel
	errors := <-collectedErrorChannel

	retErr := error(nil)
	if errors.Len() > 0 {
		retErr = errors
	}

	return results, retErr
}

// spawnWorkers spawns all workers needed for hashing. All worker results will be returned on the provided channel.
func (hasher *ParallelWalkHasher) spawnWorkers(ctx context.Context, waitGroup *sync.WaitGroup, workChan <-chan pathedData) <-chan hashResult {
	workerChannels := make([]chan hashResult, hasher.numWorkers)
	for i := 0; i < hasher.numWorkers; i++ {
		workerChannel := make(chan hashResult)
		workerChannels[i] = workerChannel
		waitGroup.Add(1)
		go func() {
			hasher.doHashWork(ctx, workChan, workerChannel)
			waitGroup.Done()
		}()
	}

	return mergeResultChannels(workerChannels)
}

// dispatchWork will send jobs to all workers through the given workChan.
func (hasher *ParallelWalkHasher) dispatchWork(ctx context.Context, work []pathedData, workChan chan<- pathedData) {
	for i, job := range work {
		// Send some work, but we may need to bail out early if the context has been cancelled.
		select {
		case workChan <- job:
			// Not the _MOST_ accurate, since we're really just reporting when work has been sent, but it's good enough.
			hasher.progressReporter.ReportProgress(Progress(i * 100 / len(work)))
		case <-ctx.Done():
		}
	}
}

// doHashWork provides all of the coordination needed for workers to process hashes.
func (hasher *ParallelWalkHasher) doHashWork(ctx context.Context, workChan <-chan pathedData, resultChan chan<- hashResult) {
	defer close(resultChan)
	for {
		select {
		case reader, ok := <-workChan:
			// If there's no work left, we're done.
			if !ok {
				return
			}

			outHash, err := hasher.processData(reader)
			result := hashResult{
				path: reader.path,
				hash: outHash,
				err:  err,
			}

			resultChan <- result
		case <-ctx.Done():
			return
		}
	}
}

// processData will perform the hash and any cleanup needed for the given reader.
func (hasher *ParallelWalkHasher) processData(reader pathedData) (hash.Hash, error) {
	outHash := hasher.constructor()
	data, err := reader.open()
	if err != nil {
		err = xerrors.Errorf("could not open data for path (%s) in worker: %w", reader.path, err)
		return nil, err
	}

	defer data.Close()
	err = hashReader(outHash, data)
	if err != nil {
		err = xerrors.Errorf("could not hash reader in worker (%s): %w", reader.path, err)
		return nil, err
	}

	return outHash, nil
}

// collectResults collects all of the results from workers, and will return it on the provided channel when complete.
func (hasher *ParallelWalkHasher) collectResults(cancelFunc context.CancelFunc, resultChan <-chan hashResult, errorChan chan<- error) <-chan PathHashes {
	outChan := make(chan PathHashes)
	go func() {
		hashes := make(PathHashes)
		for result := range resultChan {
			// If we've received an error, we should store it and move on.
			// We will cancel the context, but there are still workers that may want to finish up.
			if result.err != nil {
				errorChan <- result.err
				cancelFunc()
				continue
			}

			hashes[result.path] = result.hash
		}

		outChan <- hashes
		close(outChan)
		// We've collected all results, so we're also done collecting errors.
		close(errorChan)
	}()

	return outChan
}

// collectErrors collects all of the errors from errorChan, and will produce one MultiError on the provided channel.
func (hasher *ParallelWalkHasher) collectErrors(errorChan <-chan error) <-chan *multierror.MultiError {
	outChan := make(chan *multierror.MultiError)
	errors := multierror.NewMultiError()
	go func() {
		for err := range errorChan {
			errors.Append(err)
		}

		outChan <- errors
		close(outChan)
	}()

	return outChan
}

// mergeResultChannels will merge all channels of hashResult into a single channel.
func mergeResultChannels(workerChannels []chan hashResult) <-chan hashResult {
	outChan := make(chan hashResult)
	go func() {
		waitGroup := sync.WaitGroup{}
		for _, workerChan := range workerChannels {
			waitGroup.Add(1)
			go func(workerChan <-chan hashResult, outChan chan<- hashResult) {
				for result := range workerChan {
					outChan <- result
				}
				waitGroup.Done()
			}(workerChan, outChan)
		}

		// When we're done merging all of the results, we can safely close the channel
		waitGroup.Wait()
		close(outChan)
	}()

	return outChan
}
