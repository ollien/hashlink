package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"github.com/ollien/xtrace"
	"golang.org/x/xerrors"
)

var (
	errWrongNumberOfArguments = errors.New("wrong number of arguments")
	errInvalidNumberOfWorkers = errors.New("invalid number of workers")
	errOutDirNotEmpty         = errors.New("out_dir not empty")
)

// cliArgs rpresents the arguments that can be passed to the entrypoint command
type cliArgs struct {
	dryRun       bool
	copyMissing  bool
	numWorkers   int
	srcDir       string
	referenceDir string
	outDir       string
}

func main() {
	args, err := setupAndValidateArgs()
	if err != nil {
		handleArgsError(err, args)
		os.Exit(1)
	}

	reporter := progressBarReporter{}
	hasher := getWalkHasher(args.numWorkers, reporter)

	fmt.Println("Hashing src_dir files...")
	srcHashes, err := hasher.WalkAndHash(args.srcDir)
	if err != nil {
		reporter.abort()
		handleError(err)
		os.Exit(1)
	}

	reporter.finish()
	fmt.Println("Hashing reference_dir files...")
	referenceHashes, err := hasher.WalkAndHash(args.referenceDir)
	if err != nil {
		reporter.abort()
		handleError(err)
		os.Exit(1)
	}

	reporter.finish()
	// Create a mapping of reference files to src files
	identicalFiles := hashlink.FindIdenticalFiles(srcHashes, referenceHashes)
	missingFiles := findMissingFiles(srcHashes, referenceHashes, identicalFiles)
	fmt.Println("Done scanning.")
	if len(missingFiles) > 0 {
		missingFilesOutput, err := makeIndentedJSONOutput(missingFiles)
		if err != nil {
			err = xerrors.Errorf("could not generate missing file output: %w", err)
			handleError(err)
		}

		fmt.Printf("The following files will not be linked.\n%v\n", missingFilesOutput)
	} else {
		fmt.Print("\n")
	}

	op := getConnectFunction(args.dryRun, os.Link)
	err = connectFiles(identicalFiles, args.srcDir, args.outDir, op)
	if err != nil {
		handleError(err)
		os.Exit(1)
	}

	op = getConnectFunction(args.dryRun, copyFile)
	err = connectFiles(identicalFiles, args.srcDir, args.outDir, op)

	output := "Done processing. Enjoy your files :)"
	if args.dryRun {
		copiedFiles := []string{}
		if args.copyMissing {
			copiedFiles = missingFiles
		}

		output = getDryRunOutput(identicalFiles, copiedFiles)
	}

	fmt.Println(output)
}

// Usage specifies the usage for the cmd package
func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ./hashlink [-j n] [-n] [-c] src_dir reference_dir out_dir")
	flag.PrintDefaults()
}

func setupAndValidateArgs() (cliArgs, error) {
	args := cliArgs{}
	flag.Usage = Usage
	flag.IntVar(&args.numWorkers, "j", 1, "specify a number of workers")
	flag.BoolVar(&args.dryRun, "n", false, "do not link any files, but print out what files would have been linked")
	flag.BoolVar(&args.copyMissing, "c", false, "copy the files that are missing from either src_dir or reference_dir")
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
	if err != nil {
		return args, err
	}

	err = assertDirEmpty(args.outDir)
	if err != nil {
		return args, err
	}

	return args, nil
}

func handleArgsError(err error, args cliArgs) {
	if err == errInvalidNumberOfWorkers {
		fmt.Fprintf(os.Stderr, "Invalid number of workers (%d). Must be >= 1\n", args.numWorkers)
	} else if err == errOutDirNotEmpty {
		fmt.Fprintf(os.Stderr, "The provided out_dir (%s) is non-empty. Cowardly refusing to run.\n", args.outDir)
	} else if err != errWrongNumberOfArguments {
		// If we have errWrongNumberOfArguments, we don't need to do any special handling other than the usage string.
		fmt.Fprintln(os.Stderr, err)
	}

	Usage()
}

func handleError(err error) {
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
func getWalkHasher(numWorkers int, reporter hashlink.ProgressReporter) hashlink.WalkHasher {
	// If we only have one worker, there's no point in spinning up a parallel hash walker.
	if numWorkers > 1 {
		return hashlink.NewParallelWalkHasher(numWorkers, sha256.New, hashlink.ParallelWalkHasherProgressReporter(reporter))
	}

	return hashlink.NewSerialWalkHasher(sha256.New, hashlink.SerialWalkHasherProgressReporter(reporter))
}

// getConnectFunction gives a nop function if dryRun is try, the fallback otherwise.
func getConnectFunction(dryRun bool, fallback connectFunction) connectFunction {
	if dryRun {
		return func(src, dst string) error {
			return nil
		}
	}

	return fallback
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

func assertDirEmpty(dir string) error {
	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		return xerrors.Errorf("could not read dir contents: %w", err)
	}

	if len(contents) > 0 {
		return errOutDirNotEmpty
	}

	return nil
}

// getDryRunOutput gets the output for the termination of the program when the dryRun flag is provided.
func getDryRunOutput(identicalFiles hashlink.FileMap, copiedFiles []string) string {
	type output struct {
		Linked []string `json:"linked"`
		Copied []string `json:"copied,omitempty"`
	}

	linkedFiles := make([]string, len(identicalFiles))
	i := 0
	for file := range identicalFiles {
		linkedFiles[i] = file
		i++
	}

	out, err := makeIndentedJSONOutput(output{Linked: linkedFiles, Copied: copiedFiles})
	if err != nil {
		handleError(err)
		os.Exit(1)
	}

	return out
}

// makeIndentedJSONOutput makes a JSON formatted string of the given item
func makeIndentedJSONOutput(target interface{}) (string, error) {
	out, err := json.MarshalIndent(target, "", "\t")

	return string(out), err
}

// findMissingFiles will find all files missing in files that are in present in srcHashes or referenceHashes
// files is expected to have files from srcDir as the keys, and referenceDir as the values
func findMissingFiles(srcHashes, referenceHashes hashlink.PathHashes, files hashlink.FileMap) []string {
	// To get missing files for the refernce dir, we must make our map in order of reference => src
	flippedFiles := hashlink.MakeFlippedFileMap(files)
	missingFiles := hashlink.GetUnmappedFiles(referenceHashes, flippedFiles)
	missingSrcFiles := hashlink.GetUnmappedFiles(srcHashes, files)
	missingFiles = append(missingFiles, missingSrcFiles...)

	return missingFiles
}
