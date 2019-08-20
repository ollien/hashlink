package hashlink

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
	"hash"
	"io"

	"golang.org/x/xerrors"
)

// PathHashes represent the hashes for all paths walked by a WalkHasher, with the path as the key,
// and the hash as the value.
type PathHashes map[string]hash.Hash

// WalkHasher represents something that can walk a tree and generate hashes.
type WalkHasher interface {
	// WalkAndHash takes a root path and returns a path of each file, along with its hash.
	WalkAndHash(root string) (PathHashes, error)
}

// hashReader will hash a reader into the given hash interface.
func hashReader(h hash.Hash, reader io.Reader) (retErr error) {
	_, err := io.Copy(h, reader)
	if err != nil {
		retErr = xerrors.Errorf("could not hash file: %w", err)
	}

	return
}
