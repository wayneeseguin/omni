package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wayneeseguin/flocklogger"
)

func main() {
	// Setup log directory
	logDir := "./logs/retention"
	os.MkdirAll(logDir, 0755)

	// Create some dummy old log files for demonstration
	createOldLogFiles(logDir)

	// Create logger with time-based retention
	logger, err := flocklogger.NewFlockLogger(filepath.Join(logDir, "app.log"))
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Configure logger with short max age for demo purposes
	logger.SetMaxAge(2 * time.Minute)           // Remove logs older than 2 minutes
	logger.SetCleanupInterval(10 * time.Second) // Check every 10 seconds (for demo)
	logger.SetMaxSize(1024)                     // Small size to trigger rotation

	logger.Info("Started application with time-based log retention")
	logger.Infof("Logs older than %v will be automatically removed", 2*time.Minute)

	// Generate some logs to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Infof("Log message %d: Logs are now being managed with age-based retention", i)
		time.Sleep(100 * time.Millisecond)
	}

	// Run cleanup manually (though the background process will do this too)
	logger.Info("Running manual cleanup...")
	if err := logger.RunCleanup(); err != nil {
		logger.Errorf("Error during manual cleanup: %v", err)
	}

	// Log final status
	logger.Info("Age-based log retention example complete")
	logger.Info("Check the logs directory to see which files were retained")

	// Close the logger
	logger.Close()

	// List remaining files
	fmt.Println("\nRemaining log files after cleanup:")
	listLogFiles(logDir)
}

// createOldLogFiles creates some dummy log files with different ages
func createOldLogFiles(dir string) {
	// Clear directory first
	files, _ := os.ReadDir(dir)
	for _, file := range files {
		os.Remove(filepath.Join(dir, file.Name()))
	}

	// Create files with different ages
	createDummyLog(dir, "app.log.1", 1*time.Hour)    // 1 hour old
	createDummyLog(dir, "app.log.2", 30*time.Minute) // 30 minutes old
	createDummyLog(dir, "app.log.3", 10*time.Minute) // 10 minutes old
	createDummyLog(dir, "app.log.4", 1*time.Minute)  // 1 minute old
	createDummyLog(dir, "app.log.5", 30*time.Second) // 30 seconds old

	fmt.Println("Created dummy log files with different ages:")
	listLogFiles(dir)
	fmt.Println("\nStarting logger with 2-minute retention policy...")
}

// createDummyLog creates a log file with modified timestamp
func createDummyLog(dir, name string, age time.Duration) {
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("Error creating dummy log %s: %v\n", name, err)
		return
	}

	// Write some content
	fmt.Fprintf(f, "This is a dummy log file created for age-based retention testing\n")
	f.Close()

	// Set modification time to simulate age
	modTime := time.Now().Add(-age)
	os.Chtimes(path, modTime, modTime)
}

// listLogFiles lists all log files in the directory with their ages
func listLogFiles(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			fmt.Printf("- %s (error getting info: %v)\n", file.Name(), err)
			continue
		}

		age := time.Since(info.ModTime())
		fmt.Printf("- %s (age: %v)\n", file.Name(), age.Round(time.Second))
	}
}
