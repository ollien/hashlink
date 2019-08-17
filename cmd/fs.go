package main

import (
	"os"
	"path"
	"path/filepath"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

const defaultFileMode os.FileMode = 0755

type connectFunction = func(src, dst string) error

// connectFiles performs the given function op on all provided files, in order to form a connection between them, such
// as copying or hard linking. All of the files in the keys of fileMap must be contained within srcDir, and outDir will
// follow the same structur as srcDir. If the file does not exist in srcDir, no link will be created an error will be
// returned for that file.
func connectFiles(files hashlink.FileMap, srcDir, outDir string, op connectFunction) error {
	errors := multierror.NewMultiError()
	for file := range files {
		err := connectFile(file, srcDir, outDir, op)
		if err != nil {
			err = xerrors.Errorf("could not link file: %w", err)
			errors.Append(err)
		}
	}

	if errors.Len() > 0 {
		return errors
	}

	return nil
}

// connectFiles performs the given function op on the provided file, in order to form a connection between them, such
// as copying or hard linking. outDir will follow the same structur as srcDir. If the file does not exist in srcDir,
// no link will be created an error will be  returned for that file.
func connectFile(srcPath, srcDir, outDir string, op connectFunction) error {
	relSrcPath, err := filepath.Rel(srcDir, srcPath)
	if err != nil {
		return xerrors.Errorf("could not produce relative path for file linking - srcPath may not be contained in srcDir: %w", err)
	}

	outPath := path.Join(outDir, relSrcPath)
	// TODO: correct permissions of directory to match original
	err = ensureContainingDirsArePresent(outPath)
	if err != nil {
		return xerrors.Errorf("could not make directories for linking file (%s => %s): %w", srcPath, outPath, err)
	}

	return op(srcPath, outPath)
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
