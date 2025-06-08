# macOS Linker Warnings

## Issue

When running Go tests on macOS (especially on Apple Silicon/arm64), you may encounter linker warnings like:

```
ld: warning: '/private/var/folders/.../000012.o' has malformed LC_DYSYMTAB, 
expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
```

## Cause

These warnings are caused by a known issue with the Go toolchain on macOS when CGO is enabled. The linker (ld) detects a mismatch in the symbol table of the generated object files.

## Solution

### Option 1: Disable CGO (Recommended for tests)

Run tests with CGO disabled to avoid the warnings:

```bash
CGO_ENABLED=0 go test ./...
```

Or use the Makefile targets:

```bash
make test         # Uses CGO_ENABLED=0
make test-clean   # Explicitly runs without CGO
```

### Option 2: Ignore the warnings

These warnings are harmless and don't affect the functionality of your tests or binaries. They can be safely ignored.

### Option 3: Use specific linker flags

For production builds where CGO is required, you can suppress debug information:

```bash
go build -ldflags="-w -s" ./...
```

## When CGO is Required

Some tests, particularly those using the race detector, require CGO:

```bash
make test-race   # Will show warnings but is necessary for race detection
```

## References

- [Go Issue #61229](https://github.com/golang/go/issues/61229)
- [Apple Developer Forums Discussion](https://developer.apple.com/forums/thread/737577)