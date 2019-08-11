package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"hash"
	"os"

	"github.com/ollien/hashlink"
	"github.com/ollien/xtrace"
)

func main() {
	var numWorkers int
	flag.Usage = func() {

	}
	flag.IntVar(&numWorkers, "j", 1, "specify a number of workers")
	flag.Parse()
	if flag.NArg() != 1 {
		Usage()
		os.Exit(1)
	}

	if numWorkers <= 0 {
		fmt.Fprintln(os.Stderr, "Invalid number of workers")
		Usage()
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	var hasher hashlink.WalkHasher = hashlink.NewSerialWalkHasher(sha256.New)
	if numWorkers >= 1 {
		hasher = hashlink.NewParallelWalkHasher(numWorkers, sha256.New)
	}

	hashes, err := hasher.WalkAndHash(srcDir)
	if err != nil {
		xtrace.Trace(err)
		os.Exit(1)
	}
	printResults(hashes)
}

// Usage specifies the usage for the cmd package
func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ./hashlink src_dir")
	flag.PrintDefaults()
}

func printResults(hashes map[string]hash.Hash) {
	buffer := make([]byte, 0)
	for path, hash := range hashes {
		sum := hash.Sum(buffer)
		fmt.Printf("%s => %x\n", path, sum)
	}
}
