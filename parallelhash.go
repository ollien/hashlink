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
	constructor func() hash.Hash
	walker      pathWalker
	numWorkers  int
	errors      *multierror.MultiError
}

// hashResult represents the result of a hashing operation.
type hashResult struct {
	path string
	// hash represents the hash of the data located at path
	hash hash.Hash
	// If an error occured during operation, then err will be non-nil.
	err error
}

type parallelWalkHasherChannelSet struct {
	// all io.Readers to hash will be sent to the workers through workChan
	workChan chan pathedData
	// all resulting hashes will be sent from the workers through resultChan
	resultChan chan hashResult
}

// NewParallelWalkHasher makekes a new hash walker with a constructor for a hash algorithm and a number of workers
func NewParallelWalkHasher(numWorkers int, constructor func() hash.Hash) *ParallelWalkHasher {
	walker := fileWalker{}

	return makeParallelHashWalker(numWorkers, walker, constructor)
}

// makeParallelHashWalker will build a serial hash walker with the given spec. Used mainly as faux-dependency injection
func makeParallelHashWalker(numWorkers int, walker pathWalker, constructor func() hash.Hash) *ParallelWalkHasher {
	return &ParallelWalkHasher{
		walker:      walker,
		constructor: constructor,
		numWorkers:  numWorkers,
	}
}

// WalkAndHash walks the given path across all workers and returns hashes for all the files in the path
func (hasher *ParallelWalkHasher) WalkAndHash(root string) (PathHashes, error) {
	// the work chan may have been closed from a previous run, so we should make a new one
	channels := parallelWalkHasherChannelSet{
		workChan:   make(chan pathedData),
		resultChan: make(chan hashResult),
	}

	outMap := sync.Map{}
	collectWaitGroup := sync.WaitGroup{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	go hasher.collectResults(cancelFunc, &collectWaitGroup, channels, &outMap)

	workerWaitGroup := sync.WaitGroup{}
	hasher.spawnWorkers(ctx, &workerWaitGroup, channels)
	err := hasher.walker.Walk(root, func(reader pathedData) error {
		// Send some work, but we may need to bail out early if the context has been cancelled.
		select {
		case channels.workChan <- reader:
		case <-ctx.Done():
		}

		return nil
	})

	// whether we got an error or not, we're done supplying work
	close(channels.workChan)
	if err != nil {
		return nil, xerrors.Errorf("could not perform parallel hash walk: %w", err)
	}

	workerWaitGroup.Wait()
	collectWaitGroup.Wait()
	// Avoid issues with typed nils being returned
	retErr := error(nil)
	if hasher.errors != nil && len(hasher.errors.Errors()) > 0 {
		retErr = hasher.errors
	}

	return makePathHashesFromSyncMap(&outMap), retErr
}

// spawnWorkers spawns all workers needed for hashing
func (hasher *ParallelWalkHasher) spawnWorkers(ctx context.Context, waitGroup *sync.WaitGroup, channels parallelWalkHasherChannelSet) {
	workerChannels := make([]chan hashResult, hasher.numWorkers)
	for i := 0; i < hasher.numWorkers; i++ {
		workerChannel := make(chan hashResult)
		workerChannels[i] = workerChannel
		// Create a set of channels for this specific worker.
		workerChannelSet := parallelWalkHasherChannelSet{
			workChan:   channels.workChan,
			resultChan: workerChannel,
		}
		go func() {
			waitGroup.Add(1)
			hasher.doHashWork(ctx, workerChannelSet)
			waitGroup.Done()
		}()
	}

	go mergeResultChannels(workerChannels, channels.resultChan)
}

// doHashWork provides all of the coordination needed for workers to process hashes
func (hasher *ParallelWalkHasher) doHashWork(ctx context.Context, channels parallelWalkHasherChannelSet) {
	defer close(channels.resultChan)
	for {
		select {
		case reader, ok := <-channels.workChan:
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

			channels.resultChan <- result
		case <-ctx.Done():
			return
		}
	}
}

// collectResults collects all of the results from workers
func (hasher *ParallelWalkHasher) collectResults(cancelFunc context.CancelFunc, waitGroup *sync.WaitGroup, channels parallelWalkHasherChannelSet, outMap *sync.Map) {
	waitGroup.Add(1)
	defer waitGroup.Done()

	for result := range channels.resultChan {
		// If we've received an error, we should store it and move on.
		// We will cancel the context, but there are still workers that may want to finish up.
		if result.err != nil {
			hasher.addError(result.err)
			cancelFunc()
			continue
		}

		outMap.Store(result.path, result.hash)
	}
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
func mergeResultChannels(workerChannels []chan hashResult, outChannel chan hashResult) {
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
