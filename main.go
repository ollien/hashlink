package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"

	"github.com/ollien/xtrace"
)

const usage = "Usage: ./linker src_dir"

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	hashWalker := NewSerialHashWalker(sha256.New)
	hashes, err := hashWalker.HashWalk(srcDir)
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
