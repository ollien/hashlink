package hashlink

import "encoding/hex"

// FileMap represents a mapping between one file path and any related file paths.
type FileMap map[string][]string

// FindIdenticalFiles generates an IdenticalFileGetter that describes the identical files in hashes,
// mapped to the identical files in other.
func FindIdenticalFiles(hashes PathHashes, other PathHashes) FileMap {
	hashPaths := mapHashesToPaths(hashes)
	otherHashPaths := mapHashesToPaths(other)
	res := make(FileMap)
	for hash, paths := range hashPaths {
		otherPaths, havePaths := otherHashPaths[hash]
		if !havePaths {
			continue
		}

		for _, path := range paths {
			res[path] = append(res[path], otherPaths...)
		}
	}

	return res
}

// GetUnmappedFiles returns all files that are in hashes but not files.
func GetUnmappedFiles(hashes PathHashes, files FileMap) []string {
	intersection := []string{}
	for path := range hashes {
		_, ok := files[path]
		if !ok {
			intersection = append(intersection, path)
		}
	}

	return intersection
}

// mapHashesToPaths will flip the map, and bucket all non-unique hashes into one key, where the keys are string digests
// of the hash hash.Hashes are not compariable on their own, thus we need to encode them.
func mapHashesToPaths(hashes PathHashes) map[string][]string {
	res := make(map[string][]string)
	sum := make([]byte, 0)
	for path, hash := range hashes {
		sum = hash.Sum(sum)
		key := hex.EncodeToString(sum)
		sum = sum[:0]
		res[key] = append(res[key], path)
	}

	return res
}
