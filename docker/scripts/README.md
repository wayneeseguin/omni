# Docker Scripts

This directory contains scripts used by Docker containers for testing.

## diskfull-test

A Perl script that sets up a limited filesystem environment for testing disk full scenarios.

### Features

- Creates a 1MB tmpfs filesystem at `/test-logs`
- Sets the `OMNI_DISKFULL_TEST_PATH` environment variable
- Runs the pre-compiled test binary with disk full tests
- Displays filesystem usage before and after tests
- Lists files created during testing with sizes and permissions
- Exits with the same code as the test binary for CI integration

### Implementation Details

The script is written in Perl using only the standard library for maximum portability:

- Uses `system()` for executing external commands
- Uses `POSIX` module for proper exit code handling
- Uses `opendir/readdir` for directory listing
- Uses `stat()` for file information

### Usage

This script is automatically executed when running the disk full Docker container:

```bash
docker run --rm \
    --privileged \
    --cap-add SYS_ADMIN \
    -e OMNI_DISKFULL_TEST_PATH=/test-logs \
    omni/diskfull-test:latest
```

### Requirements

- Perl 5 (included in Alpine Linux by default)
- Privileged container mode for mounting tmpfs
- SYS_ADMIN capability for mount operations

### Exit Codes

The script exits with the same code as the test binary:
- 0: All tests passed
- Non-zero: Tests failed or execution error occurred