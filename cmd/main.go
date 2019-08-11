package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"

	"github.com/ollien/hashlink"
	"github.com/ollien/xtrace"
)

const usage = "Usage: ./hashlink src_dir"

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	hasher := hashlink.NewParallelWalkHasher(2, sha256.New)
	hashes, err := hasher.WalkAndHash(srcDir)
	if err != nil {
		xtrace.Trace(err)
		os.Exit(1)
	}

	buffer := make([]byte, 0)
	for path, hash := range hashes {
		sum := hash.Sum(buffer)
		fmt.Printf("%s => %x\n", path, sum)
	}
}
