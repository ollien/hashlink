package main

/*
	Copyright 2019 Nicholas Krichevsky

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/ollien/hashlink"
	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

const defaultFileMode os.FileMode = 0755

type connectFunction = func(src, dst string) error

// connectMappedFiles performs the given function op on all provided files (expected in src => reference order), in
// order to form a connection between them, such as copying or hardlinking. If the file in the value portion of files
// does not exist in referenceDir,  no connection will be created an error will be returned for that file, but
// connecting will continue for all other files.
func connectMappedFiles(files hashlink.FileMap, referenceDir, outDir string, op connectFunction) error {
	errors := multierror.NewMultiError()
	for srcFile, referenceFiles := range files {
		for _, referenceFile := range referenceFiles {
			err := connectMappedFile(srcFile, referenceFile, referenceDir, outDir, op)
			if err != nil {
				err = xerrors.Errorf("could not link file (%s): %w", srcFile, err)
				errors.Append(err)
			}
		}
	}

	if errors.Len() > 0 {
		return errors
	}

	return nil
}

// connectMappedFile will connect srcPath to a file in outDir that is relative to outDir in the same fashion that
// referencePath is relative to referenceDir. If referencePath is not relative to referenceDir, an error is returned
// and no connection will take place.
func connectMappedFile(srcPath, referencePath, referenceDir, outDir string, op connectFunction) error {
	relReferencePath, err := filepath.Rel(referenceDir, referencePath)
	if err != nil {
		return xerrors.Errorf("could not produce relative path for file connection: %w", err)
	}

	outPath := path.Join(outDir, relReferencePath)
	err = op(srcPath, outPath)
	if err != nil {
		return xerrors.Errorf("could not connect path (%s => %s): %w", srcPath, outPath)
	}

	return nil
}

// connectFiles performs the given function op on all provided files, in order to form a connection between them, such
// as copying or hardlinking. If the file does not exist in baseDir, an error will be returned for that file, but
// connecting will continue for all other files.
func connectFiles(files []string, baseDir, outDir string, op connectFunction) error {
	errors := multierror.NewMultiError()
	for _, file := range files {
		err := connectFile(file, baseDir, outDir, op)
		if err != nil {
			err = xerrors.Errorf("could not link file (%s): %w", file, err)
			errors.Append(err)
		}
	}

	if errors.Len() > 0 {
		return errors
	}

	return nil
}

// connectFile will connect path to a file in outDir that is relative to outDir in the same fashion that it is
// relative to baseDir. If the given path is not relative to srcDir, an error is returned and no connection will
// take place.
func connectFile(path, baseDir, outDir string, op connectFunction) error {
	// This looks a little silly, but path can safely act as our "referencePath", as we are still operating relative
	// to it.
	return connectMappedFile(path, path, baseDir, outDir, op)
}

// ensureContainingDirsArePresent ensures that the dirs needed for a file are fully present. Will make the directories
// if needed. All file modes will be defaultFileMode, and should be corrected by the caller if anything else is desired.
func ensureContainingDirsArePresent(filePath string) error {
	dirComponent := path.Dir(filePath)
	err := os.MkdirAll(dirComponent, defaultFileMode)
	if err != nil {
		return xerrors.Errorf("could not make directories for file (%s): %w", filePath, err)
	}

	return nil
}

// copyFile copies a file from src to dst. Both paths must be regular files.
// (for some reason the standard library includes no way to do this out of the box...)
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return xerrors.Errorf("could not open file (%s) for copying: %w", srcFile, err)
	}

	createMode := removeExecuteBits(defaultFileMode)
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, createMode)
	if err != nil {
		return xerrors.Errorf("could not open path (%s) as copying destination: %w", dst, err)
	}

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return xerrors.Errorf("could noy copy (%s => %s): %w", src, dst, err)
	}

	return nil
}

// removeExecuteBits will remove the execute bits from the given FileMode
func removeExecuteBits(mode os.FileMode) os.FileMode {
	mask := ^os.FileMode(0111)

	return mode & mask
}
