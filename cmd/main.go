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
	if flag.NArg() != 1 {
		Usage()
		os.Exit(1)
	} else if numWorkers <= 0 {
		fmt.Fprintln(os.Stderr, "Invalid number of workers")
		Usage()
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	// If we only have one worker, there's no point in spinning up a parallel hash walker.
	var hasher hashlink.WalkHasher = hashlink.NewSerialWalkHasher(sha256.New)
	if numWorkers >= 1 {
		hasher = hashlink.NewParallelWalkHasher(numWorkers, sha256.New)
	}

	hashes, err := hasher.WalkAndHash(srcDir)
	if err != nil {
		// Some hash walkers make use of MultiErrors, so we should try to unpack those first if we can.
		multiErr, isMulti := err.(*multierror.MultiError)
		if isMulti {
			for _, singleError := range multiErr.Errors() {
				xtrace.Trace(singleError)
			}
		} else {
			xtrace.Trace(err)
		}

		os.Exit(1)
	}

	printResults(hashes)
}

// Usage specifies the usage for the cmd package
func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ./hashlink [-j n] src_dir")
	flag.PrintDefaults()
}

func printResults(hashes hashlink.PathHashes) {
	buffer := make([]byte, 0)
	for path, hash := range hashes {
		sum := hash.Sum(buffer)
		fmt.Printf("%s => %x\n", path, sum)
	}
}
