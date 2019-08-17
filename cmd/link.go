package main

import (
	"os"
	"path"
	"path/filepath"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

// linkFiles will hardlink all paths within filePaths to outDir in the same configuration as they were in srcDir
// If the file does not exist in srcDir, no link will be created an error will be returned for that file.
func linkFiles(files hashlink.FileMap, srcDir, outDir string) error {
	errors := multierror.NewMultiError()
	for file := range files {
		err := linkFile(file, srcDir, outDir)
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
