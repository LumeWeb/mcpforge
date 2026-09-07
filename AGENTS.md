# AGENTS.md

This file provides development guidelines and architectural documentation for
the mcpforge project.

## Common Commands

### Building
```bash
# Build all packages
go build -v ./...
```

### Testing
```bash
# Run all tests with race detection and coverage
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Run tests for the root package only
go test -v -race .

# View coverage report
go tool cover -func=coverage.out
```

### Templated Code Generation

```bash
# Generate mocks for interfaces (uses .mockery.yaml; mockery is
# pre-installed at $HOME/go/bin/mockery)
mockery
```

Mocks are pre-generated and committed to the repository. NEVER reinstall
mockery. Run `mockery` with no arguments after adding interfaces to
`.mockery.yaml` or to the root package, then commit the regenerated files.

### Dependency Management
```bash
# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Tidy dependencies
go mod tidy
```

## Project Overview

mcpforge is a context-generic DSL for conditionally constructing
descriptions, lists, JSON schemas, tool targets, and agent guides, resolved
headlessly over a caller-defined context type.

## Layout

- **Module path**: `go.lumeweb.com/mcpforge`
- **Root package** (`mcpforge`): all core functionality lives in repo-root
  `.go` files; `doc.go` holds the package documentation
- **Subdirectories** exist only for real subpackages — never for organizational
  convenience
- **Tests** are colocated with the code they test (same directory, `_test.go`
  suffix)
- **Generated mocks** live in `mocks/` (mockery, testify templates) and are
  committed to the repository

## Design Rules

The package is a pure script layer and must stay that way:

- No MCP SDK imports
- No MCP host detection or host-environment inspection
- No product-specific content

Builders accept caller-supplied predicates and resolve against a generic
context type supplied at the call site.

## Conventions

- Do not add a README/board copyright header to source files; attribution
  lives only in the LICENSE file
- Keep the public API described in `doc.go` consistent with `README.md`
