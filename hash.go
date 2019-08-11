package main

import (
	"context"
	"errors"
	"hash"
	"io"
	"log"
	"sync"

	"golang.org/x/xerrors"
)

var errNotFile = errors.New("not a file; will not hash")

// WalkHasher represents something that can walk a tree and generate hashes
type WalkHasher interface {
	// WalkAndHash takes a root path and returns a path of each file, along with its hash
	WalkAndHash(root string) (map[string]hash.Hash, error)
}

// SerialWalkHasher will hash all files one after the other
// Implements HashWalker
type SerialWalkHasher struct {
	constructor func() hash.Hash
	walker      pathWalker
}

// ParallelWalkHasher will hash all files concurrently, up to the number of specified workers
type ParallelWalkHasher struct {
	constructor func() hash.Hash
	walker      pathWalker
	numWorkers  int
}

type parallelWalkHasherChannelSet struct {
	// all io.Readers to hash will be sent to the workers through workChan
	workChan chan pathedReader
	// all resulting hashes will be sent from the workers through hashChan
	hashChan chan pathedHash
	// all resulting errors will be sent from the workers through errorChan
	errorChan chan error
}

// pathedHash represents a hash associated with a path
type pathedHash struct {
	path string
	hash hash.Hash
}

// hashReader will hash a reader into the given hash interface, h
func hashReader(h hash.Hash, reader io.Reader) (retErr error) {
	_, err := io.Copy(h, reader)
	if err != nil {
		retErr = xerrors.Errorf("could not hash file: %w", err)
	}

	return
}

// NewSerialWalkHasher makes a new serial hasher with a constructor for a hash algorithm
func NewSerialWalkHasher(constructor func() hash.Hash) SerialWalkHasher {
	walker := fileWalker{}

	return makeSerialHashWalker(walker, constructor)
}

// makeSerialHashWalker will build a serial hash walker with the given spec. Used mainly as faux-dependency injection
func makeSerialHashWalker(walker pathWalker, constructor func() hash.Hash) SerialWalkHasher {
	return SerialWalkHasher{
		walker:      walker,
		constructor: constructor,
	}
}

// WalkAndHash walks the given path and returns hashes for all the files in the path
func (hasher SerialWalkHasher) WalkAndHash(root string) (map[string]hash.Hash, error) {
	walkedMap := make(map[string]hash.Hash)
	// Walk all of the files and collect hashes for them
	err := hasher.walker.Walk(root, func(reader pathedReader) error {
		outHash := hasher.constructor()
		err := hashReader(outHash, reader.reader)
		if err != nil {
			return xerrors.Errorf("could not hash path (%s): %w", reader.path, err)
		}

		walkedMap[reader.path] = outHash
		return nil
	})

	if err != nil {
		return nil, xerrors.Errorf("could not perform serial hash walk: %w", err)
	}

	return walkedMap, nil
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
		workChan:  make(chan pathedReader),
		hashChan:  make(chan pathedHash),
		errorChan: make(chan error),
	}

	outMap := sync.Map{}
	collectWaitGroup := sync.WaitGroup{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	go hasher.collectResults(&collectWaitGroup, cancelFunc, channels, &outMap)
	go collectErrors(channels.errorChan)

	workerWaitGroup := sync.WaitGroup{}
	hasher.spawnWorkers(ctx, channels, &workerWaitGroup, func(reader io.Reader) (hash.Hash, error) {
		outHash := hasher.constructor()
		err := hashReader(outHash, reader)
		if err != nil {
			return nil, xerrors.Errorf("could not hash reader in worker: %w", err)
		}

		return outHash, nil
	})

	err := hasher.walker.Walk(root, func(reader pathedReader) error {
		channels.workChan <- reader

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
func (hasher ParallelWalkHasher) spawnWorkers(ctx context.Context, channels parallelWalkHasherChannelSet, waitGroup *sync.WaitGroup, process func(io.Reader) (hash.Hash, error)) {
	for i := 0; i < hasher.numWorkers; i++ {
		go func() {
			waitGroup.Add(1)
			hasher.doHashWork(ctx, channels, process)
			waitGroup.Done()
		}()
	}
}

// doHashWork provides all of the coordination needed for workers to process hashes
func (hasher ParallelWalkHasher) doHashWork(ctx context.Context, channels parallelWalkHasherChannelSet, process func(io.Reader) (hash.Hash, error)) {
	done := false
	for !done {
		select {
		case reader, ok := <-channels.workChan:
			// If there's no work left, we're done.
			if !ok {
				done = true
				break
			}

			outHash, err := process(reader.reader)
			if err != nil {
				channels.errorChan <- err
				continue
			}

			result := pathedHash{
				path: reader.path,
				hash: outHash,
			}
			channels.hashChan <- result
		case <-ctx.Done():
			done = true
		}
	}
}

// collectResults collects all of the results from workers
func (hasher ParallelWalkHasher) collectResults(waitGroup *sync.WaitGroup, cancelFunc context.CancelFunc, channels parallelWalkHasherChannelSet, outMap *sync.Map) error {
	waitGroup.Add(1)
	defer waitGroup.Done()

	done := false
	for !done {
		select {
		case result, ok := <-channels.hashChan:
			// If we have no more values to recieve, then we're done.
			if !ok {
				done = true
				break
			}

			outMap.Store(result.path, result.hash)
		case err := <-channels.errorChan:
			cancelFunc()
			return err
		}
	}

	return nil
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

// collectErrors will collect all errors from the given error channel and log them
func collectErrors(errorChan chan error) {
	for err := range errorChan {
		log.Println(err)
	}
}
