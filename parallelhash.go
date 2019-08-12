package hashlink

import (
	"context"
	"errors"
	"hash"
	"sync"

	"golang.org/x/xerrors"
)

// ParallelWalkHasher will hash all files concurrently, up to the number of specified workers
type ParallelWalkHasher struct {
	constructor func() hash.Hash
	walker      pathWalker
	numWorkers  int
	errors      []error
	errorLock   sync.RWMutex
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
	go hasher.collectResults(ctx, cancelFunc, &collectWaitGroup, channels, &outMap)

	workerWaitGroup := sync.WaitGroup{}
	hasher.spawnWorkers(ctx, &workerWaitGroup, channels)
	err := hasher.walker.Walk(root, func(reader pathedData) error {
		select {
		case channels.workChan <- reader:
		case <-ctx.Done():
			return errors.New("one of the workers experienced an error - work could not be sent")
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
	err = hasher.generateWalkError()

	return makePathHashesFromSyncMap(&outMap), err
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
func (hasher *ParallelWalkHasher) collectResults(ctx context.Context, cancelFunc context.CancelFunc, waitGroup *sync.WaitGroup, channels parallelWalkHasherChannelSet, outMap *sync.Map) {
	waitGroup.Add(1)
	defer waitGroup.Done()

	for {
		select {
		case result, ok := <-channels.resultChan:
			// If we have no more values to recieve, then we're done.
			if !ok {
				return
			} else if result.err != nil {
				// If we've received an erorr, we should store it and move on.
				hasher.errorLock.Lock()
				hasher.errors = append(hasher.errors, result.err)
				hasher.errorLock.Unlock()
				continue
			}

			outMap.Store(result.path, result.hash)
		case <-ctx.Done():
			return
		}
	}
}

// Errors will return all errors that occured during walking.
func (hasher *ParallelWalkHasher) Errors() []error {
	errorsCopy := make([]error, len(hasher.errors))
	hasher.errorLock.RLock()
	defer hasher.errorLock.RUnlock()
	copy(errorsCopy, hasher.errors)

	return errorsCopy
}

// generateWalkErrors generates an error if any have occured during the hashing process
func (hasher *ParallelWalkHasher) generateWalkError() (err error) {
	hasher.errorLock.RLock()
	defer hasher.errorLock.RUnlock()
	if len(hasher.errors) > 0 {
		err = errors.New("errors occured during hashing; check .Errors()")
	}

	return
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
