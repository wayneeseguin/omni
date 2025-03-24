# Flock Logger ("flocklogger")

A Go library for logging with flock-based synchronization on Unix-like systems.

**Note:** This library uses `unix.Flock` and is intended for use on Unix-like systems (e.g., Linux, macOS).

The primary use case of this is to allow for background logging to a single log from multiple processes without blocking without clobbering each other.

## Installation

To install `flocklogger`, use the following command:

```sh
go get github.com/wayneeseguin/flocklogger
```

## Usage

Here's a simple example of how to use `flocklogger`:

```go
package main

import (
    "github.com/wayneeseguin/flocklogger"
)

func main() {
    logger, err := flocklogger.NewFlockLogger("app.log")
    if err != nil {
        panic(err)
    }
    defer logger.Close()

    logger.Flog("Hello, %s!", "world")
}
```

For more examples, see the [examples](examples) directory.

## Testing

To run the tests, use the following command:

```sh
go test
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

