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

	"github.com/ollien/hashlink/multierror"
	"golang.org/x/xerrors"
)

const defaultFileMode os.FileMode = 0755

type connectFunction = func(src, dst string) error

// connectFiles performs the given function op on all provided files, in order to form a connection between them, such
// as copying or hard linking. outDir will follow the same structure as srcDir. If the file does not exist in srcDir,
// no connection will be created an error will be returned for that file, but connecting will continue for all other
// files.
func connectFiles(files []string, srcDir, outDir string, op connectFunction) error {
	errors := multierror.NewMultiError()
	for _, file := range files {
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

	return op(srcPath, outPath)
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
