# FastGoGenerate

FastGoGenerate is a tool designed to speed up Go code generation by caching generated files and only regenerating them when necessary.

## Overview

The tool acts as a wrapper around Go code generation tools like `mockgen` and `gowrap`, providing caching capabilities to avoid unnecessary regeneration of files that haven't changed.

### Performance Comparison

| Run Type | Command | Time | Speed Improvement |
|----------|---------|------|-------------------|
| Before | `make pr` | 3:04 | Baseline |
| First Run | `FASTGOGENERATE_ENABLED=true make pr` | 3:02 | 1.0x |
| Second Run | `FASTGOGENERATE_ENABLED=true make pr` | 0:36 | 5.1x |

## How It Works
1. When a code generation command is executed, fastgogenerate intercepts it
2. It computes a unique ID (sha256) for the generation based on input parameters:
   * Arguments
   * Input file contents
   * Output file names
3. Checks if the output already exists in the cache
4. If cached, skips a plugin tool execution
5. If not cached, runs the original tool and caches the result

## Limitations

FastGoGenerate cannot track all dependencies used by tools due to the complexity and customized dependencies, so it cannot provide a 100% guarantee of change detection. To ensure correctness, we have a `buildkite` job that performs the same operations without any caching to verify the results.

## Usage

### As a Binary

You can use FastGoGenerate directly in your `go:generate` directives by installing it as a binary:

```bash
go install github.com/uber/cadence/cmd/tools/fastgogenerate
```

Then modify your `go:generate` directives to use `fastgogenerate` instead of the original tool:

```go
// Before
//go:generate mockgen -package $GOPACKAGE -source queryParser.go -destination queryParser_mock.go -mock_names Interface=MockQueryParser

// After
//go:generate fastgogenerate mockgen -package $GOPACKAGE -source queryParser.go -destination queryParser_mock.go -mock_names Interface=MockQueryParser
```

The tool will automatically handle caching and only regenerate files when necessary.

### Makefile

FastGoGenerate is an experimental tool and is not enabled by default.

#### How to Enable FastGoGenerate

To enable FastGoGenerate, follow these steps:
1. Run `make clean` to delete all binaries and caches
2. Run `make install-fastgogenerated-tools` to install fastgogenerated tools
3. Run `make pr` or `make go-generate` to run the code generation with FastGoGenerate
4. The first run will take the same amount of time as without caching, but subsequent runs should be faster

#### How to Disable FastGoGenerate

To disable FastGoGenerate, follow these steps:
1. Run `make clean` to delete all binaries and caches
2. Run `make pr` or `make go-generate` to install original binaries and run the code generation with original binaries

#### How It Works

* When `make install-fastgogenerated-tools` is run, the Makefile builds a fastgogenerate binary with a built-in plugin name that replaces the original plugin binary
* The original plugin binary is stored with the same name plus `.bin` suffix
* The Makefile sets `FASTGOGENERATE_CACHE_PATH` to `$(shell pwd)/$(BUILD)/.fastgogenerate_cache` to ensure that the cache can be easily removed with `make clean`

### Environment Variables

The tool can be configured using the following environment variables:

| Variable | Description | Default Value |
|----------|-------------|---------------|
| `FASTGOGENERATE_DEBUG` | Enable debug logging | `false` |
| `FASTGOGENERATE_CACHE_DISABLED` | Disable caching | `false` |
| `FASTGOGENERATE_CACHE_PATH` | Path to store the cache | `os.TempDir()` |
| `FASTGOGENERATE_BINARY_SUFFIX_DISABLED` | Disable binary suffix | `false` |
| `FASTGOGENERATE_TIMEOUT` | Timeout for code generation | `10s` |
| `FASTGOGENERATE_ORIGINAL_BINARY_SUFFIX` | Suffix for original binaries | `bin` |
| `FASTGOGENERATE_MOCKGEN_BINARY_PATH` | Custom path for mockgen binary | `"mockgen"` |
| `FASTGOGENERATE_GOWRAP_BINARY_PATH` | Custom path for gowrap binary | `"gowrap"` |

### Supported Plugins

Currently, the tool supports the following code generation tools:
- mockgen (go.uber.org/mock/mockgen)
  * Package mode is not supported (WIP)
  * Plugin adds `-write_command_comment=false` option to ensure that generation command is not mentioned in mock files
- gowrap (github.com/hexdigest/gowrap/cmd/gowrap)

### Contributing

To add support for new code generation tools:
1. Create a new plugin in the `plugins` directory
2. Implement the required plugin interface
3. Register the plugin in the plugin registry via `InitializePlugins` function
4. Update the Makefile to use the new plugin via `go_build_tool_or_fastgogenerated_tool`

### Similar Projects

- [go-generate-fast](https://github.com/oNaiPs/go-generate-fast)