package main

import (
	"context"
	"errors"
	"hash"
	"io"
	"sync"

	"github.com/ollien/xtrace"
	"golang.org/x/xerrors"
)

// ParallelWalkHasher will hash all files concurrently, up to the number of specified workers
type ParallelWalkHasher struct {
	constructor func() hash.Hash
	walker      pathWalker
	numWorkers  int
}

type parallelWalkHasherChannelSet struct {
	// all io.Readers to hash will be sent to the workers through workChan
	workChan chan pathedData
	// all resulting hashes will be sent from the workers through hashChan
	hashChan chan pathedHash
	// all resulting errors will be sent from the workers through errorChan
	errorChan chan error
}

// NewParallelWalkHasher makekes a new hash walker with a constructor for a hash algorithm and a number of workers
func NewParallelWalkHasher(numWorkers int, constructor func() hash.Hash) ParallelWalkHasher {
	walker := fileWalker{}

	return makeParallelHashWalker(numWorkers, walker, constructor)
}

// makeParallelHashWalker will build a serial hash walker with the given spec. Used mainly as faux-dependency injection
func makeParallelHashWalker(numWorkers int, walker pathWalker, constructor func() hash.Hash) ParallelWalkHasher {
	return ParallelWalkHasher{
		walker:      walker,
		constructor: constructor,
		numWorkers:  numWorkers,
	}
}

// WalkAndHash walks the given path across all workers and returns hashes for all the files in the path
func (hasher ParallelWalkHasher) WalkAndHash(root string) (map[string]hash.Hash, error) {
	// the work chan may have been closed from a previous run, so we should make a new one
	channels := parallelWalkHasherChannelSet{
		workChan:  make(chan pathedData),
		hashChan:  make(chan pathedHash),
		errorChan: make(chan error),
	}

	outMap := sync.Map{}
	collectWaitGroup := sync.WaitGroup{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	go hasher.collectResults(ctx, &collectWaitGroup, channels, &outMap)
	go collectErrors(channels.errorChan, cancelFunc)

	workerWaitGroup := sync.WaitGroup{}
	hasher.spawnWorkers(ctx, channels, &workerWaitGroup, func(data io.ReadCloser) (hash.Hash, error) {
		outHash := hasher.constructor()
		defer data.Close()
		err := hashReader(outHash, data)
		if err != nil {
			return nil, xerrors.Errorf("could not hash reader in worker: %w", err)
		}

		return outHash, nil
	})

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
	close(channels.hashChan)
	collectWaitGroup.Wait()
	close(channels.errorChan)

	return hasher.convertSyncMapToResultMap(&outMap), nil
}

// spawnWorkers spawns all workers needed for hashing
func (hasher ParallelWalkHasher) spawnWorkers(ctx context.Context, channels parallelWalkHasherChannelSet, waitGroup *sync.WaitGroup, process func(io.ReadCloser) (hash.Hash, error)) {
	for i := 0; i < hasher.numWorkers; i++ {
		go func() {
			waitGroup.Add(1)
			hasher.doHashWork(ctx, channels, process)
			waitGroup.Done()
		}()
	}
}

// doHashWork provides all of the coordination needed for workers to process hashes
func (hasher ParallelWalkHasher) doHashWork(ctx context.Context, channels parallelWalkHasherChannelSet, process func(io.ReadCloser) (hash.Hash, error)) {
	for {
		select {
		case reader, ok := <-channels.workChan:
			// If there's no work left, we're done.
			if !ok {
				return
			}

			outHash, err := process(reader.data)
			if err != nil {
				channels.errorChan <- err
				continue
			}

			result := pathedHash{
				path: reader.path,
				hash: outHash,
			}

			// Send on the channel if we can, but we may need to bail out early.
			select {
			case channels.hashChan <- result:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// collectResults collects all of the results from workers
func (hasher ParallelWalkHasher) collectResults(ctx context.Context, waitGroup *sync.WaitGroup, channels parallelWalkHasherChannelSet, outMap *sync.Map) {
	waitGroup.Add(1)
	defer waitGroup.Done()

	for {
		select {
		case result, ok := <-channels.hashChan:
			// If we have no more values to recieve, then we're done.
			if !ok {
				return
			}

			outMap.Store(result.path, result.hash)
		case <-ctx.Done():
			return
		}
	}
}

func (hasher ParallelWalkHasher) convertSyncMapToResultMap(syncMap *sync.Map) map[string]hash.Hash {
	resultMap := make(map[string]hash.Hash)
	syncMap.Range(func(key, value interface{}) bool {
		// We should panic if these values aren't the correct type - there is no way we can produce a result that is correct
		keyStr := key.(string)
		valueHash := value.(hash.Hash)
		resultMap[keyStr] = valueHash

		return true
	})

	return resultMap
}

// collectErrors will collect all errors from the given error channel and log them, then cancel the context for the pool
func collectErrors(errorChan chan error, cancelFunc context.CancelFunc) {
	for err := range errorChan {
		xtrace.Trace(err)
		cancelFunc()
	}
}
