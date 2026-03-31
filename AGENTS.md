# AGENTS.md - Coding Guidelines for go-plugin

## Project Overview

`github.com/hashicorp/go-plugin` is a Go library for building plugin systems using
process isolation with net/rpc or gRPC transport. Plugins are standalone binaries
that communicate with a host process. Go 1.24, MPL-2.0 license.

## Build / Lint / Test Commands

```bash
# Build all packages
go build ./...

# Run all tests with race detection
go test -race ./...

# Run all tests with verbose output and coverage
go test -race ./... -v -coverprofile=coverage.out

# Run a single test
go test -race -run TestClient -v .
go test -race -run TestServer_testMode -v ./...

# Run tests in a specific package
go test -race -v ./internal/cmdrunner

# Lint
golangci-lint run

# Format check (CI fails if files are changed)
go fmt ./...

# Regenerate protobuf code (requires buf)
buf generate --path test/grpc/test.proto
```

## Code Style

### File Headers

Every `.go` file must start with the copyright header:

```go
// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0
```

### Package Declarations

- Root package is `plugin`.
- Internal packages live under `internal/` (e.g., `internal/cmdrunner`,
  `internal/grpcmux`, `internal/plugin`).
- Public sub-packages: `runner/`, `test/grpc/`.

### Imports

Group imports in three blocks separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages (`github.com/hashicorp/go-plugin/...`)

Use named imports sparingly; only when disambiguation is needed:

```go
import (
    "context"
    "fmt"

    hclog "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/go-plugin/internal/grpcmux"
    "google.golang.org/grpc"
)
```

### Naming Conventions

- Exported types and functions use PascalCase: `Client`, `ServeConfig`, `HandshakeConfig`.
- Unexported uses camelCase: `managedClients`, `defaultPluginLogBufferSize`.
- Interfaces define behavior; use `-er` suffix where appropriate (`Runner`).
- Test helpers accept `testing.TB` (not `*testing.T`) to support both tests and benchmarks.
- Prefix test functions with `Test`: `TestClient`, `TestServer_testMode`.
- Use underscores in test names for readability: `TestClient_killStart`.

### Error Handling

- Return errors; do not panic in library code.
- Define sentinel errors as package-level `var` using `errors.New`:
  ```go
  var ErrProcessNotFound = cmdrunner.ErrProcessNotFound
  var ErrChecksumsDoNotMatch = errors.New("checksums did not match")
  ```
- Wrap errors with `fmt.Errorf("context: %w", err)` when adding context.
- Discard cleanup errors with `_ =`: `_ = l.Close()`.

### Types and Interfaces

- Struct fields are documented with comments above each field.
- Use interface compliance checks in test files:
  ```go
  var _ Plugin = (*testInterfacePlugin)(nil)
  var _ Plugin = new(NetRPCUnsupportedPlugin)
  ```
- Prefer embedding for composition: `NetRPCUnsupportedPlugin` is designed for embedding.

### Testing Patterns

- Tests are in `*_test.go` files in the same package (not `_test` package).
- Use `t.Fatal` / `t.Fatalf` for unexpected errors, not `t.Error`.
- Use `t.TempDir()` for temporary directories; clean up with `defer`.
- Use `context.WithCancel` with `defer cancel()` for context-based tests.
- Helper processes are spawned via `helperProcess()` in `plugin_test.go`.

### Logging

- Use `github.com/hashicorp/go-hclog` for all logging.
- Prefer `hclog.New()` with explicit `LoggerOptions` in non-test code.
- Logger is passed via `ClientConfig.Logger` or created with sensible defaults.

### gRPC / Protobuf

- Proto definitions in `test/grpc/test.proto`.
- Generated code uses `buf` with settings in `buf.yaml` / `buf.gen.yaml`.
- Do not edit generated `*.pb.go` files directly.
