package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"github.com/ollien/xtrace"
	"golang.org/x/xerrors"
)

const defaultFileMode os.FileMode = 0755

func main() {
	var numWorkers int
	flag.Usage = Usage
	flag.IntVar(&numWorkers, "j", 1, "specify a number of workers")
	flag.Parse()
	if flag.NArg() != 3 {
		Usage()
		os.Exit(1)
	} else if numWorkers <= 0 {
		fmt.Fprintln(os.Stderr, "Invalid number of workers")
		Usage()
		os.Exit(1)
	}

	srcDir := flag.Arg(0)
	referenceDir := flag.Arg(1)
	outDir := flag.Arg(2)
	// If we only have one worker, there's no point in spinning up a parallel hash walker.
	var hasher hashlink.WalkHasher = hashlink.NewSerialWalkHasher(sha256.New)
	if numWorkers > 1 {
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

	// Create a mapping of reference files to src files
	identicalFiles := hashlink.FindIdenticalFiles(referenceHashes, srcHashes)
	missingFiles := hashlink.GetUnmappedFiles(referenceHashes, identicalFiles)
	fmt.Print("Done scanning.")
	if len(missingFiles) > 0 {
		fmt.Printf("The following files will not be processed.\n%v\n", missingFiles)
	} else {
		fmt.Print("\n")
	}

	err = linkFiles(identicalFiles, srcDir, outDir)
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

func printResults(fileMap hashlink.FileMap) {
	for path, matchingFiles := range fileMap {
		fmt.Printf("%s => %v\n", path, matchingFiles)
	}
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

// linkFiles will hardlink all paths within filePaths to outDir in the same configuration as they were in srcDir
// If the file does not exist in srcDir, no link will be created an error will be returned for that file.
func linkFiles(files hashlink.FileMap, srcDir, outDir string) error {
	errors := multierror.NewMultiError()
	for _, identicalFiles := range files {
		// TODO: This is a hack - we should implement a flip for FileMap
		for _, file := range identicalFiles {
			err := linkFile(file, srcDir, outDir)
			if err != nil {
				err = xerrors.Errorf("could not link file: %w", err)
				errors.Append(err)
			}
		}
	}

	if errors.Len() > 0 {
		return errors
	}

	return nil
}

// linkFile takes a file and will make a hardlink within the destination directory in the same configuration as it was
// in srcDir. If the file does not exist in srcDir, no link will be created an error will be returned for that file.
func linkFile(srcPath, srcDir, destDir string) error {
	relSrcPath, err := filepath.Rel(srcDir, srcPath)
	if err != nil {
		return xerrors.Errorf("could not produce relative path for file linking - srcPath may not be contained in srcDir: %w", err)
	}

	outPath := path.Join(destDir, relSrcPath)
	// TODO: correct permissions of directory to match original
	err = ensureContainingDirsArePresent(outPath)
	if err != nil {
		return xerrors.Errorf("could not make directories for linking file (%s => %s): %w", srcPath, outPath, err)
	}

	return os.Link(srcPath, outPath)
}

// ensureContainignDirsArepResent ensures that the dirs needed for a file are fully present. Will make the directories
// if needed. All file modes will be defaultFileMode, and should be corrected by the caller if anything else is desired.
func ensureContainingDirsArePresent(filePath string) error {
	dirComponent := path.Dir(filePath)
	err := os.MkdirAll(dirComponent, defaultFileMode)
	if err != nil {
		return xerrors.Errorf("could not make directories for file (%s): %w", filePath, err)
	}

	return nil
}
