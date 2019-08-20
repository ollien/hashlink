package main

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
	"sync"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
)

// dirResult represents the result of generting the hashes of a directory
type dirResult struct {
	dir    string
	hashes hashlink.PathHashes
	err    error
}

// getHashes will get all of the hashes needed from the given directories
func getHashes(srcDir, referenceDir string, numWorkers int) (srcHashes hashlink.PathHashes, referenceHashes hashlink.PathHashes, err error) {
	reporter := progressBarReporter{}
	reporterAggregator := newProgressReporterAggregator(reporter, 2)

	srcChan := getHashesForDir(srcDir, numWorkers, reporterAggregator)
	referenceChan := getHashesForDir(referenceDir, numWorkers, reporterAggregator)
	resultChan := mergeResultChannels(srcChan, referenceChan)
	// Store our hashes in a map based on directory so we can get the proper return result
	hashes := make(map[string]hashlink.PathHashes, 2)
	errors := multierror.NewMultiError()
	for result := range resultChan {
		hashes[result.dir] = result.hashes
		if result.err != nil {
			errors.Append(result.err)
		}
	}

	// avoid returns with type nils by specifying our nil error here
	retErr := error(nil)
	if errors.Len() > 0 {
		retErr = errors
		reporter.abort()
	} else {
		reporter.finish()
	}

	return hashes[srcDir], hashes[referenceDir], retErr
}

// getHashesForDir will get all of the hashes for the given dir, and report them onto the provided channel
func getHashesForDir(dir string, numWorkers int, aggregator *progressReporterAggregator) <-chan dirResult {
	resultChan := make(chan dirResult)
	go func() {
		reporter := newSubAggregateProgressReporter(aggregator)
		hasher := getWalkHasher(numWorkers, reporter)
		hashes, err := hasher.WalkAndHash(dir)
		resultChan <- dirResult{
			dir:    dir,
			hashes: hashes,
			err:    err,
		}

		close(resultChan)
	}()

	return resultChan
}

// mergeResultChannels will merge all channels of hashResult into a single channel
func mergeResultChannels(resultChannels ...<-chan dirResult) <-chan dirResult {
	outChan := make(chan dirResult)
	go func() {
		waitGroup := sync.WaitGroup{}
		for _, resultChan := range resultChannels {
			waitGroup.Add(1)
			go func(resultChan <-chan dirResult, outChan chan<- dirResult) {
				for result := range resultChan {
					outChan <- result
				}
				waitGroup.Done()
			}(resultChan, outChan)
		}

		// When we're done merging all of the results, we can safely close the channel
		waitGroup.Wait()
		close(outChan)
	}()

	return outChan
}
