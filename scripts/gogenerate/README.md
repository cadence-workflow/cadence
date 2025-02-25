# GoGenerate

This folder contains custom scripts designed to replace the original binaries invoked during `go generate` commands.

## Why do we need these scripts?

In many cases, regenerating files during `go generate` is unnecessary if the source files (or other dependencies) have
not been updated.
These scripts introduce a simple optimization: they check file modification times and skip regeneration if the output
file is already up-to-date.
This saves time and computational resources, especially in large projects.

## How It Works

1. Each script should be called by `make go-generate` or `make pr`
    1. Directly by `go generate` command will not work, because it will call the original binary that is not replaced by
       the script
2. Each script intercepts the call to its respective binary (e.g., `mockgen`, `gowrap`) during `go generate`.
3. Before running the original command, the script checks:
    - If the destination file is newer than its dependencies (e.g., source files, templates).
    - If so, it skips regeneration and exits early.
4. If regeneration is necessary (i.e., dependencies are newer), the script executes the original binary with all
   provided arguments.

## How to Debug

1. To enable debug logs run `GO_GENERATE_SCRIPTS_DEBUG=true make go generate ./...`.
2. Be sure that your script support this flag

## Add a new script

1. Save a new script in this folder with a name matching its corresponding binary (e.g., `mockgen.sh`, `gowrap.sh`).
2. Make them executable: `chmod +x script.sh`.
3. Run `make clean` to remove the old binaries.

A new script will automatically replace the original binary in `./build/bin` when running `make go-generate`
or `make pr`.
`Makefile` will rename original binary to `<binary>.local` and copy the new script to `./build/bin`.

## Example Scripts

### `mockgen.sh`

Replaces the `mockgen` binary. It checks if the generated mock file is newer than the source file (`$GOFILE`). If so, it
skips regeneration.

### `gowrap.sh`

Replaces the `gowrap` binary. It ensures that the destination file is newer than both `$GOFILE` and the template file
before running the original `gowrap` command.
