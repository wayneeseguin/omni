package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wayneeseguin/flexlog"
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
var logger *flexlog.FlexLog

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
	level := flexlog.LevelInfo
	if *debug {
		level = flexlog.LevelDebug
	} else if *verbose {
		level = flexlog.LevelTrace
	}
	
	// Build logger
	builder := flexlog.NewBuilder().
		WithPath(logPath).
		WithLevel(level).
		WithRotation(10*1024*1024, 5) // 10MB files, keep 5
	
	if *jsonLogs {
		builder = builder.WithJSON()
	} else {
		builder = builder.WithText()
	}
	
	// Create logger
	var err error
	logger, err = builder.Build()
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	
	// Add console output for warnings and errors
	logger.AddDestination("stderr://")
	
	// Log startup information
	logger.WithFields(map[string]interface{}{
		"version":   "1.0.0",
		"log_level": flexlog.LevelName(level),
		"log_file":  logPath,
		"pid":       os.Getpid(),
	}).Info("Application started")
	
	return nil
}

func processFiles(files []string) error {
	logger.WithField("count", len(files)).Info("Starting file processing")
	
	processed := 0
	errors := 0
	startTime := time.Now()
	
	for i, file := range files {
		fileLogger := logger.WithFields(map[string]interface{}{
			"file":  file,
			"index": i + 1,
			"total": len(files),
		})
		
		fileLogger.Debug("Processing file")
		
		// Simulate file processing
		if err := processFile(file); err != nil {
			fileLogger.WithError(err).Error("Failed to process file")
			errors++
			continue
		}
		
		processed++
		
		// Log progress every 10 files
		if processed%10 == 0 {
			progress := float64(processed) / float64(len(files)) * 100
			fileLogger.WithFields(map[string]interface{}{
				"processed": processed,
				"progress":  fmt.Sprintf("%.1f%%", progress),
			}).Info("Processing progress")
		}
	}
	
	// Log summary
	duration := time.Since(startTime)
	logger.WithFields(map[string]interface{}{
		"processed":   processed,
		"errors":      errors,
		"duration_ms": duration.Milliseconds(),
		"rate":        fmt.Sprintf("%.2f files/sec", float64(processed)/duration.Seconds()),
	}).Info("Processing completed")
	
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
	
	logger.WithFields(map[string]interface{}{
		"size":     info.Size(),
		"modified": info.ModTime(),
	}).Trace("File details")
	
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
		stepLogger := logger.WithFields(map[string]interface{}{
			"step":     i + 1,
			"total":    len(steps),
			"activity": step,
		})
		
		stepLogger.Info("Analysis step started")
		
		// Simulate work
		time.Sleep(500 * time.Millisecond)
		
		// Log some debug information
		if logger.IsLevelEnabled(flexlog.LevelDebug) {
			stepLogger.WithField("memory_mb", getMemoryUsage()).
				Debug("Step resource usage")
		}
		
		stepLogger.Info("Analysis step completed")
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
		logger.WithError(err).Error("Failed to create report file")
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
		logger.WithField("section", section).Debug("Writing report section")
		
		_, err := fmt.Fprintf(file, "## %s\n\n", section)
		if err != nil {
			logger.WithError(err).
				WithField("section", section).
				Error("Failed to write section")
			return fmt.Errorf("write section %s: %w", section, err)
		}
		
		// Simulate content generation
		time.Sleep(200 * time.Millisecond)
	}
	
	logger.WithField("path", reportPath).Info("Report generated successfully")
	fmt.Printf("Report saved to: %s\n", reportPath)
	
	return nil
}

func getMemoryUsage() int {
	// Simplified memory usage (would use runtime.MemStats in real app)
	return 42
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
			logger.WithField("panic", r).Error("Application panicked")
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
		logger.WithField("operation", *operation).Error("Invalid operation")
	}
	
	if err != nil {
		logger.WithError(err).Error("Operation failed")
		os.Exit(1)
	}
	
	logger.Info("Operation completed successfully")
}