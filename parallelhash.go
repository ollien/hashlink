package hashlink

import (
	"context"
	"hash"
	"sync"

	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

// ParallelWalkHasher will hash all files concurrently, up to the number of specified workers
type ParallelWalkHasher struct {
	constructor      func() hash.Hash
	walker           pathWalker
	numWorkers       int
	errors           *multierror.MultiError
	progressReporter ProgressReporter
}

// hashResult represents the result of a hashing operation.
type hashResult struct {
	path string
	// hash represents the hash of the data located at path
	hash hash.Hash
	// If an error occured during operation, then err will be non-nil.
	err error
}

// ParallelWalkHasherProgressReporter will provide a ProgressReporter for a ParallelWalkWasher.
// Intended to be passed to NewParallelWalkHasher as an option
func ParallelWalkHasherProgressReporter(reporter ProgressReporter) func(*ParallelWalkHasher) {
	return func(hasher *ParallelWalkHasher) {
		hasher.progressReporter = reporter
	}
}

// NewParallelWalkHasher makekes a new hash walker with a constructor for a hash algorithm and a number of workers
func NewParallelWalkHasher(numWorkers int, constructor func() hash.Hash, options ...func(*ParallelWalkHasher)) *ParallelWalkHasher {
	walker := fileWalker{}

	return makeParallelHashWalker(numWorkers, walker, constructor, options...)
}

// makeParallelHashWalker will build a serial hash walker with the given spec. Used mainly as faux-dependency injection
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

// WalkAndHash walks the given path across all workers and returns hashes for all the files in the path
func (hasher *ParallelWalkHasher) WalkAndHash(root string) (PathHashes, error) {
	// Reset the errors from the last run
	// TODO: This is not concurrency safe
	hasher.errors = nil
	workChan := make(chan pathedData)
	resultChan := make(chan hashResult)

	ctx, cancelFunc := context.WithCancel(context.Background())
	collectedResultChannel := hasher.collectResults(cancelFunc, resultChan)

	workerWaitGroup := sync.WaitGroup{}
	walkerItems, err := getAllItemsFromWalker(hasher.walker, root)
	if err != nil {
		return nil, xerrors.Errorf("could not perform get items for parallel hash walk: %w", err)
	}

	hasher.spawnWorkers(ctx, &workerWaitGroup, workChan, resultChan)
	hasher.dispatchWork(ctx, walkerItems, workChan)
	close(workChan)
	workerWaitGroup.Wait()
	results := <-collectedResultChannel
	// Avoid issues with typed nils being returned
	retErr := error(nil)
	if hasher.errors != nil && hasher.errors.Len() > 0 {
		retErr = hasher.errors
	}

	return results, retErr
}

// spawnWorkers spawns all workers needed for hashing
func (hasher *ParallelWalkHasher) spawnWorkers(ctx context.Context, waitGroup *sync.WaitGroup, workChan <-chan pathedData, resultChan chan<- hashResult) {
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

	go mergeResultChannels(workerChannels, resultChan)
}

// dispatchWork will send jobs to all workers through the given workChan
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

// doHashWork provides all of the coordination needed for workers to process hashes
func (hasher *ParallelWalkHasher) doHashWork(ctx context.Context, workChan <-chan pathedData, resultChan chan<- hashResult) {
	defer close(resultChan)
	for {
		select {
		case reader, ok := <-workChan:
			// If there's no work left, we're done.
			if !ok {
				return
			}

			outHash := hasher.constructor()
			defer reader.data.Close()
			err := hashReader(outHash, reader.data)
			if err != nil {
				err = xerrors.Errorf("could not hash reader in worker: %w", err)
			}

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

// collectResults collects all of the results from workers, and will return it on the provided channel when complete.
func (hasher *ParallelWalkHasher) collectResults(cancelFunc context.CancelFunc, resultChan <-chan hashResult) <-chan PathHashes {
	outChan := make(chan PathHashes)
	go func() {
		hashes := make(PathHashes)
		for result := range resultChan {
			// If we've received an error, we should store it and move on.
			// We will cancel the context, but there are still workers that may want to finish up.
			if result.err != nil {
				hasher.addError(result.err)
				cancelFunc()
				continue
			}

			hashes[result.path] = result.hash
		}

		outChan <- hashes
		close(outChan)
	}()

	return outChan
}

// addError will setup the MultiError if needed, and append an error to it.
func (hasher *ParallelWalkHasher) addError(err error) {
	if hasher.errors == nil {
		hasher.errors = multierror.NewMultiError(err)
		return
	}

	hasher.errors.Append(err)
}

// mergeResultChannels will merge all channels of hashResult into a single channel
func mergeResultChannels(workerChannels []chan hashResult, outChannel chan<- hashResult) {
	waitGroup := sync.WaitGroup{}
	for _, workerChan := range workerChannels {
		waitGroup.Add(1)
		go func(workerChan chan hashResult) {
			for result := range workerChan {
				outChannel <- result
			}
			waitGroup.Done()
		}(workerChan)
	}

	// When we're done merging all of the results, we can safely close the channel
	waitGroup.Wait()
	close(outChannel)
}
