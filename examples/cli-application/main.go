package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wayneeseguin/omni"
)

// CLI flags
var (
	verbose   = flag.Bool("verbose", false, "Enable verbose logging")
	debug     = flag.Bool("debug", false, "Enable debug logging")
	logFile   = flag.String("log-file", "", "Log file path (default: ~/.myapp/app.log)")
	jsonLogs  = flag.Bool("json", false, "Use JSON format for logs")
	operation = flag.String("op", "process", "Operation to perform: process, analyze, or report")
)

// Application logger
var logger *omni.Omni

func setupLogger() error {
	// Determine log file path
	logPath := *logFile
	if logPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		
		logDir := filepath.Join(homeDir, ".myapp", "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("create log directory: %w", err)
		}
		
		logPath = filepath.Join(logDir, "app.log")
	}
	
	// Determine log level
	level := omni.LevelInfo
	if *debug {
		level = omni.LevelDebug
	} else if *verbose {
		level = omni.LevelTrace
	}
	
	// Create logger with options
	options := []omni.Option{
		omni.WithPath(logPath),
		omni.WithLevel(level),
		omni.WithRotation(10*1024*1024, 5), // 10MB files, keep 5
	}
	
	if *jsonLogs {
		options = append(options, omni.WithJSON())
	} else {
		options = append(options, omni.WithText())
	}
	
	// Create logger
	var err error
	logger, err = omni.NewWithOptions(options...)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	
	// Log startup information
	logger.InfoWithFields("Application started", map[string]interface{}{
		"version":   "1.0.0",
		"log_level": getLevelName(level),
		"log_file":  logPath,
		"pid":       os.Getpid(),
	})
	
	return nil
}

func processFiles(files []string) error {
	logger.InfoWithFields("Starting file processing", map[string]interface{}{
		"count": len(files),
	})
	
	processed := 0
	errors := 0
	startTime := time.Now()
	
	for i, file := range files {
		logFields := map[string]interface{}{
			"file":  file,
			"index": i + 1,
			"total": len(files),
		}
		
		logger.DebugWithFields("Processing file", logFields)
		
		// Simulate file processing
		if err := processFile(file); err != nil {
			errorFields := map[string]interface{}{
				"file":  file,
				"index": i + 1,
				"total": len(files),
				"error": err.Error(),
			}
			logger.ErrorWithFields("Failed to process file", errorFields)
			errors++
			continue
		}
		
		processed++
		
		// Log progress every 10 files
		if processed%10 == 0 {
			progress := float64(processed) / float64(len(files)) * 100
			progressFields := map[string]interface{}{
				"file":      file,
				"index":     i + 1,
				"total":     len(files),
				"processed": processed,
				"progress":  fmt.Sprintf("%.1f%%", progress),
			}
			logger.InfoWithFields("Processing progress", progressFields)
		}
	}
	
	// Log summary
	duration := time.Since(startTime)
	logger.InfoWithFields("Processing completed", map[string]interface{}{
		"processed":   processed,
		"errors":      errors,
		"duration_ms": duration.Milliseconds(),
		"rate":        fmt.Sprintf("%.2f files/sec", float64(processed)/duration.Seconds()),
	})
	
	if errors > 0 {
		return fmt.Errorf("failed to process %d files", errors)
	}
	
	return nil
}

func processFile(path string) error {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	
	logger.TraceWithFields("File details", map[string]interface{}{
		"size":     info.Size(),
		"modified": info.ModTime(),
	})
	
	// Simulate processing
	time.Sleep(50 * time.Millisecond)
	
	// Simulate occasional errors
	if info.Size() == 0 {
		return fmt.Errorf("empty file")
	}
	
	return nil
}

func analyzeData() error {
	logger.Info("Starting data analysis")
	
	// Simulate analysis steps
	steps := []string{
		"Loading data",
		"Validating schema",
		"Computing statistics",
		"Generating insights",
		"Creating visualizations",
	}
	
	for i, step := range steps {
		stepFields := map[string]interface{}{
			"step":     i + 1,
			"total":    len(steps),
			"activity": step,
		}
		
		logger.InfoWithFields("Analysis step started", stepFields)
		
		// Simulate work
		time.Sleep(500 * time.Millisecond)
		
		// Log some debug information
		if logger.GetLevel() <= omni.LevelDebug {
			debugFields := map[string]interface{}{
				"step":      i + 1,
				"total":     len(steps),
				"activity":  step,
				"memory_mb": getMemoryUsage(),
			}
			logger.DebugWithFields("Step resource usage", debugFields)
		}
		
		logger.InfoWithFields("Analysis step completed", stepFields)
	}
	
	logger.Info("Analysis completed successfully")
	return nil
}

func generateReport() error {
	logger.Info("Generating report")
	
	reportPath := filepath.Join(os.TempDir(), "report.txt")
	
	// Create report file
	file, err := os.Create(reportPath)
	if err != nil {
		logger.ErrorWithFields("Failed to create report file", map[string]interface{}{
			"error": err.Error(),
			"path":  reportPath,
		})
		return fmt.Errorf("create report: %w", err)
	}
	defer file.Close()
	
	// Write report content
	sections := []string{
		"Executive Summary",
		"Detailed Findings",
		"Recommendations",
		"Appendix",
	}
	
	for _, section := range sections {
		logger.DebugWithFields("Writing report section", map[string]interface{}{
			"section": section,
		})
		
		_, err := fmt.Fprintf(file, "## %s\n\n", section)
		if err != nil {
			logger.ErrorWithFields("Failed to write section", map[string]interface{}{
				"section": section,
				"error":   err.Error(),
			})
			return fmt.Errorf("write section %s: %w", section, err)
		}
		
		// Simulate content generation
		time.Sleep(200 * time.Millisecond)
	}
	
	logger.InfoWithFields("Report generated successfully", map[string]interface{}{
		"path": reportPath,
	})
	fmt.Printf("Report saved to: %s\n", reportPath)
	
	return nil
}

func getMemoryUsage() int {
	// Simplified memory usage (would use runtime.MemStats in real app)
	return 42
}

func getLevelName(level int) string {
	switch level {
	case omni.LevelTrace:
		return "TRACE"
	case omni.LevelDebug:
		return "DEBUG"
	case omni.LevelInfo:
		return "INFO"
	case omni.LevelWarn:
		return "WARN"
	case omni.LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func main() {
	flag.Parse()
	
	// Setup logger
	if err := setupLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("Application shutting down")
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()
	
	// Handle panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorWithFields("Application panicked", map[string]interface{}{
				"panic": r,
			})
			panic(r) // Re-panic after logging
		}
	}()
	
	// Execute operation
	var err error
	switch *operation {
	case "process":
		// Get files from remaining arguments
		files := flag.Args()
		if len(files) == 0 {
			logger.Warn("No files specified, using test data")
			files = []string{"test1.txt", "test2.txt", "test3.txt"}
		}
		err = processFiles(files)
		
	case "analyze":
		err = analyzeData()
		
	case "report":
		err = generateReport()
		
	default:
		err = fmt.Errorf("unknown operation: %s", *operation)
		logger.ErrorWithFields("Invalid operation", map[string]interface{}{
			"operation": *operation,
		})
	}
	
	if err != nil {
		logger.ErrorWithFields("Operation failed", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	
	logger.Info("Operation completed successfully")
}