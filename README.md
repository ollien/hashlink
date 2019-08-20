# hashlink

Hashlink is a utility designed to perform migrations of duplicated data in a set of drives. Specifically, it is designed
to free up space when one file is duplicated between drives, even when their filenames differ. Hashlink will take all
matching files and hardlink them to the given destination location. Hashlink makes heavy use of concurrency to split up
the workload of hashing these files.

## Building
Run `make build` in the project root to produce the `hashlink` binary.

## Usage
```
Usage: ./hashlink [-j n] [-n] [-c] src_dir reference_dir out_dir
  -c	copy the files that are missing from src_dir
  -j int
    	specify a number of workers (default 1)
  -n	do not link any files, but print out what files would have been linked
```
Hashlink has three directories it references.

* `src_dir` is the directory from which files will be hardlinked.
* `reference_dir` is where the potentially duplicated data is stored. In the workflow that hashlink is designed to
  handle, this is located on a separate filesystem than `src_dir` or `out_dir`. If `-c` is specified, any files that
  are located within `reference_dir` but not `src_dir` will be copied from `reference_dir`.
* `out_dir` is where any hardlinks or copies will be placed. Due to the nature of how hardlinks work, this _must_ be on
  the same filesystem as `src_dir`. In addition, this directory must be empty before running the utility.

### Example Use-Case

Consider the following setup
```
$ ls -l /mnt/drive1/foo
total 0
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 a
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 b
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 c

$ ls -l /mnt/drive2/foo
total 0
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 the-same-as-a-but-different-name
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 b
-rw-r--r-- 1 nick nick 0 Aug 20 00:21 someotherfile
```

If a user desires to create `/mnt/drive1/bar`, which contains the contents shared between `/mnt/drive1/foo` and
`/mnt/drive2/foo`, running `hashlink /mnt/drive1/foo /mnt/drive2/foo /mnt/drive1/bar` will perform the desired
migration.