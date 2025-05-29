package flexlog_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/wayneeseguin/flexlog"
)

// Global variables to prevent compiler optimizations
var (
	benchmarkResult interface{}
	benchmarkError  error
)

// Benchmark basic logging operations
func BenchmarkLogLevels(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set to debug level to ensure all messages are processed
	logger.SetLevel(flexlog.LevelDebug)

	benchmarks := []struct {
		name string
		fn   func()
	}{
		{"Debug", func() { logger.Debug("benchmark debug message") }},
		{"Info", func() { logger.Info("benchmark info message") }},
		{"Warn", func() { logger.Warn("benchmark warn message") }},
		{"Error", func() { logger.Error("benchmark error message") }},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bm.fn()
			}
			logger.Sync() // Ensure all messages are processed
		})
	}
}

// Benchmark formatted logging
func BenchmarkFormattedLogging(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	b.Run("Infof", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Infof("User %s performed action %s with result %d", "alice", "login", 200)
		}
		logger.Sync()
	})

	b.Run("Debugf", func(b *testing.B) {
		logger.SetLevel(flexlog.LevelDebug)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Debugf("Debug info: counter=%d, status=%s", i, "active")
		}
		logger.Sync()
	})
}

// Benchmark structured logging
func BenchmarkStructuredLogging(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	fields := map[string]interface{}{
		"user_id":    12345,
		"action":     "purchase",
		"amount":     99.99,
		"currency":   "USD",
		"items":      3,
		"session_id": "abc-123-def",
	}

	b.Run("StructuredLog", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.StructuredLog(flexlog.LevelInfo, "transaction processed", fields)
		}
		logger.Sync()
	})

	b.Run("InfoWithFields", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.InfoWithFields("transaction processed", fields)
		}
		logger.Sync()
	})
}

// Benchmark concurrent logging
func BenchmarkConcurrentLogging(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	goroutines := []int{1, 2, 4, 8, 16, 32}

	for _, g := range goroutines {
		b.Run(fmt.Sprintf("Goroutines-%d", g), func(b *testing.B) {
			b.ResetTimer()

			var wg sync.WaitGroup
			wg.Add(g)

			messagesPerGoroutine := b.N / g

			for i := 0; i < g; i++ {
				go func(id int) {
					defer wg.Done()
					for j := 0; j < messagesPerGoroutine; j++ {
						logger.Infof("Goroutine %d message %d", id, j)
					}
				}(i)
			}

			wg.Wait()
			logger.Sync()
		})
	}
}

// Benchmark different formats
func BenchmarkFormats(b *testing.B) {
	benchmarks := []struct {
		name   string
		format flexlog.LogFormat
	}{
		{"Text", flexlog.FormatText},
		{"JSON", flexlog.FormatJSON},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			tempFile := filepath.Join(b.TempDir(), "bench.log")
			logger, err := flexlog.New(tempFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.CloseAll()

			_ = logger.SetFormat(int(bm.format))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.Info("benchmark message")
			}
			logger.Sync()
		})
	}
}

// Benchmark filtering performance
func BenchmarkFiltering(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a simple filter
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= flexlog.LevelInfo
	})

	b.Run("PassingFilter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("this passes the filter")
		}
		logger.Sync()
	})

	b.Run("FailingFilter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Debug("this fails the filter")
		}
		logger.Sync()
	})
}

// Benchmark sampling strategies
func BenchmarkSampling(b *testing.B) {
	strategies := []struct {
		name     string
		strategy flexlog.SamplingStrategy
		rate     float64
	}{
		{"None", flexlog.SamplingNone, 1.0},
		{"Random-50%", flexlog.SamplingRandom, 0.5},
		{"Random-10%", flexlog.SamplingRandom, 0.1},
		{"Consistent-50%", flexlog.SamplingConsistent, 0.5},
		{"Interval-10", flexlog.SamplingInterval, 10},
	}

	for _, s := range strategies {
		b.Run(s.name, func(b *testing.B) {
			tempFile := filepath.Join(b.TempDir(), "bench.log")
			logger, err := flexlog.New(tempFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.CloseAll()

			_ = logger.SetSampling(int(s.strategy), s.rate)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.Infof("sampled message %d", i)
			}
			logger.Sync()
		})
	}
}

// Benchmark file rotation
func BenchmarkRotation(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set small file size to trigger rotation
	logger.SetMaxSize(1024) // 1KB

	// Create a message that's about 100 bytes
	message := "This is a benchmark message for rotation testing with some padding"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(message)
		if i%10 == 0 {
			logger.Sync() // Periodic sync to actually trigger rotation
		}
	}
	logger.Sync()
}

// Benchmark compression
func BenchmarkCompression(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Enable compression
	logger.SetCompression(flexlog.CompressionGzip)
	logger.SetMaxSize(1024) // 1KB to trigger rotation

	message := "Benchmark message for compression testing"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(message)
		if i%20 == 0 {
			logger.Sync()
		}
	}
	logger.Sync()
}

// Benchmark multiple destinations
func BenchmarkMultipleDestinations(b *testing.B) {
	destinations := []int{1, 2, 3, 5}

	for _, numDest := range destinations {
		b.Run(fmt.Sprintf("Destinations-%d", numDest), func(b *testing.B) {
			// Create primary logger
			tempFile := filepath.Join(b.TempDir(), "bench-0.log")
			logger, err := flexlog.New(tempFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.CloseAll()

			// Add additional destinations
			for i := 1; i < numDest; i++ {
				destFile := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.log", i))
				if err := logger.AddDestination(destFile); err != nil {
					b.Fatalf("Failed to add destination: %v", err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.Info("message to multiple destinations")
			}
			logger.Sync()
		})
	}
}

// Benchmark memory allocations
func BenchmarkAllocations(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	b.Run("SimpleMessage", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("simple message")
		}
		logger.Sync()
	})

	b.Run("FormattedMessage", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Infof("formatted %s with %d", "message", i)
		}
		logger.Sync()
	})

	b.Run("StructuredMessage", func(b *testing.B) {
		fields := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.StructuredLog(flexlog.LevelInfo, "structured", fields)
		}
		logger.Sync()
	})
}

// Benchmark channel operations
func BenchmarkChannelSize(b *testing.B) {
	// Test different channel sizes via environment variable
	originalSize := os.Getenv("FLEXLOG_CHANNEL_SIZE")
	defer os.Setenv("FLEXLOG_CHANNEL_SIZE", originalSize)

	sizes := []string{"10", "100", "1000", "10000"}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("ChannelSize-%s", size), func(b *testing.B) {
			os.Setenv("FLEXLOG_CHANNEL_SIZE", size)

			tempFile := filepath.Join(b.TempDir(), "bench.log")
			logger, err := flexlog.New(tempFile)
			if err != nil {
				b.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.CloseAll()

			b.ResetTimer()

			// Log messages in bursts
			for i := 0; i < b.N; i++ {
				logger.Info("burst message")
				if i%100 == 0 {
					logger.Sync()
				}
			}
			logger.Sync()
		})
	}
}

// Benchmark discard logger (baseline)
func BenchmarkDiscard(b *testing.B) {
	// Create a logger that writes to discard
	logger, err := flexlog.New("/dev/null")
	if err != nil {
		// Fallback for non-Unix systems
		tempFile := filepath.Join(b.TempDir(), "bench.log")
		logger, err = flexlog.New(tempFile)
		if err != nil {
			b.Fatalf("Failed to create logger: %v", err)
		}
		// Override the writer to discard
		for _, dest := range logger.Destinations {
			dest.Writer = nil
			dest.File = nil
		}
	}
	defer logger.CloseAll()

	b.Run("Info", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("discarded message")
		}
		logger.Sync()
	})

	b.Run("Infof", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Infof("discarded %s %d", "message", i)
		}
		logger.Sync()
	})
}

// Benchmark error logging with stack traces
func BenchmarkErrorLogging(b *testing.B) {
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	testErr := fmt.Errorf("benchmark error")

	b.Run("WithoutStackTrace", func(b *testing.B) {
		logger.EnableStackTraces(false)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.ErrorWithError("error occurred", testErr)
		}
		logger.Sync()
	})

	b.Run("WithStackTrace", func(b *testing.B) {
		logger.EnableStackTraces(true)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.ErrorWithError("error occurred", testErr)
		}
		logger.Sync()
	})
}

// Helper to create a no-op writer for baseline benchmarks
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// Benchmark raw write performance (baseline)
func BenchmarkRawWrite(b *testing.B) {
	message := []byte("[2025-05-27 12:00:00.000] [INFO] benchmark message\n")

	b.Run("File", func(b *testing.B) {
		tempFile := filepath.Join(b.TempDir(), "bench.log")
		f, err := os.Create(tempFile)
		if err != nil {
			b.Fatalf("Failed to create file: %v", err)
		}
		defer f.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f.Write(message)
		}
		f.Sync()
	})

	b.Run("Discard", func(b *testing.B) {
		w := io.Discard

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Write(message)
		}
	})
}
