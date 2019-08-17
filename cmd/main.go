package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"github.com/ollien/xtrace"
	"golang.org/x/xerrors"
)

const defaultFileMode os.FileMode = 0755

var (
	errWrongNumberOfArguments = errors.New("wrong number of arguments")
	errInvalidNumberOfWorkers = errors.New("invalid number of workers")
)

// cliArgs rpresents the arguments that can be passed to the entrypoint command
type cliArgs struct {
	numWorkers   int
	srcDir       string
	referenceDir string
	outDir       string
}

func main() {
	args, err := setupAndValidateArgs()
	if err != nil {
		handleMultiError(err)
		os.Exit(1)
	}

	hasher := getWalkHasher(args.numWorkers)

	srcHashes, err := hasher.WalkAndHash(args.srcDir)
	if err != nil {
		// Some hash walkers make use of MultiErrors, so we should try to unpack those first if we can.
		handleMultiError(err)
		os.Exit(1)
	}

	referenceHashes, err := hasher.WalkAndHash(args.referenceDir)
	if err != nil {
		handleMultiError(err)
		os.Exit(1)
	}

	// Create a mapping of reference files to src files
	identicalFiles := hashlink.FindIdenticalFiles(referenceHashes, srcHashes)
	missingFiles := hashlink.GetUnmappedFiles(referenceHashes, identicalFiles)
	fmt.Print("Done scanning.")
	if len(missingFiles) > 0 {
		fmt.Printf("The following files will not be processed.\n%v\n", missingFiles)
	} else {
		fmt.Print("\n")
	}

	err = linkFiles(identicalFiles, args.srcDir, args.outDir)
	if err != nil {
		handleMultiError(err)
		os.Exit(1)
	}

	fmt.Println("Done linking. Enjoy your files :)")
}

// Usage specifies the usage for the cmd package
func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ./hashlink [-j n] src_dir reference_dir out_dir")
	flag.PrintDefaults()
}

func setupAndValidateArgs() (cliArgs, error) {
	args := cliArgs{}
	flag.Usage = Usage
	flag.IntVar(&args.numWorkers, "j", 1, "specify a number of workers")
	flag.Parse()
	if flag.NArg() != 3 {
		return cliArgs{}, errWrongNumberOfArguments
	} else if args.numWorkers <= 0 {
		return cliArgs{}, errInvalidNumberOfWorkers
	}

	args.srcDir = flag.Arg(0)
	args.referenceDir = flag.Arg(1)
	args.outDir = flag.Arg(2)
	err := assertDirsExist(args.srcDir, args.referenceDir, args.outDir)

	return args, err
}

func handleArgsError(err error) {
	if err == errInvalidNumberOfWorkers {
		fmt.Fprintf(os.Stderr, "Invalid number of workers. Must be >= 1")
	} else if err != errWrongNumberOfArguments {
		// If we have errWrongNumberOfArguments, we don't need to do any special handling other than the usage string.
		fmt.Fprintln(os.Stderr, err)
	}

	Usage()
}

func handleMultiError(err error) {
	multiErr, isMulti := err.(*multierror.MultiError)
	if isMulti {
		for i, singleError := range multiErr.Errors() {
			xtrace.Trace(singleError)
			if i != multiErr.Len()-1 {
				fmt.Fprintf(os.Stderr, "\n")
			}
		}
	} else {
		xtrace.Trace(err)
	}
}

// getWalkHasher gets the approrpiate WalkHasher based on the number of workers
func getWalkHasher(numWorkers int) hashlink.WalkHasher {
	// If we only have one worker, there's no point in spinning up a parallel hash walker.
	if numWorkers > 1 {
		return hashlink.NewParallelWalkHasher(numWorkers, sha256.New)
	}

	return hashlink.NewSerialWalkHasher(sha256.New)
}

// assertDirsExist will return true if all of the paths in the values of the map exist.
// The keys of the map should map to the name of the directory to be put into the error
func assertDirsExist(dirs ...string) error {
	errors := multierror.NewMultiError()
	for _, dir := range dirs {
		fileInfo, err := os.Stat(dir)
		if err != nil && os.IsNotExist(err) {
			err = fmt.Errorf("%s does not exist", dir)
			errors.Append(err)
		} else if err != nil {
			err = xerrors.Errorf("failed to get file info about %s: %w", dir, err)
			errors.Append(err)
		} else if !fileInfo.IsDir() {
			err := fmt.Errorf("%s is not a directory", dir)
			errors.Append(err)
		}
	}

	if errors.Len() > 0 {
		return errors
	}

	return nil
}
