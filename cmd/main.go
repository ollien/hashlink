package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"github.com/ollien/xtrace"
)

func main() {
	var numWorkers int
	flag.Usage = Usage
	flag.IntVar(&numWorkers, "j", 1, "specify a number of workers")
	flag.Parse()
	if flag.NArg() != 2 {
		Usage()
		os.Exit(1)
	} else if numWorkers <= 0 {
		fmt.Fprintln(os.Stderr, "Invalid number of workers")
		Usage()
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	referenceDir := flag.Arg(1)
	// If we only have one worker, there's no point in spinning up a parallel hash walker.
	var hasher hashlink.WalkHasher = hashlink.NewSerialWalkHasher(sha256.New)
	if numWorkers >= 1 {
		hasher = hashlink.NewParallelWalkHasher(numWorkers, sha256.New)
	}

	srcHashes, err := hasher.WalkAndHash(srcDir)
	if err != nil {
		// Some hash walkers make use of MultiErrors, so we should try to unpack those first if we can.
		handleMultiError(err)
		os.Exit(1)
	}

	referenceHashes, err := hasher.WalkAndHash(referenceDir)
	if err != nil {
		handleMultiError(err)
		os.Exit(1)
	}

	identicalFiles := hashlink.FindIdenticalFiles(srcHashes, referenceHashes)
	printResults(identicalFiles)
}

// Usage specifies the usage for the cmd package
func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ./hashlink [-j n] src_dir reference_dir")
	flag.PrintDefaults()
}

func printResults(fileMap hashlink.FileMap) {
	for path, matchingFiles := range fileMap {
		fmt.Printf("%s => %v\n", path, matchingFiles)
	}
}

func handleMultiError(err error) {
	multiErr, isMulti := err.(*multierror.MultiError)
	if isMulti {
		for _, singleError := range multiErr.Errors() {
			xtrace.Trace(singleError)
		}
	} else {
		xtrace.Trace(err)
	}
}
