package flexlog

import (
	"io"
	"sync"
)

// AddDestination adds a new log destination
func (f *FlexLog) AddDestination(name string, writer io.Writer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if destination with this name already exists
	for i, dest := range f.destinations {
		if dest.Name == name {
			// Update existing destination
			f.destinations[i].Writer = writer
			f.destinations[i].Enabled = true
			return
		}
	}

	// Add new destination
	f.destinations = append(f.destinations, LogDestination{
		Writer:  writer,
		Name:    name,
		Enabled: true,
	})
}

// RemoveDestination removes a log destination by name
func (f *FlexLog) RemoveDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.Name == name {
			// Remove by replacing with last element and truncating slice
			lastIdx := len(f.destinations) - 1
			f.destinations[i] = f.destinations[lastIdx]
			f.destinations = f.destinations[:lastIdx]
			return true
		}
	}

	return false
}

// EnableDestination enables a previously added destination
func (f *FlexLog) EnableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.Name == name {
			f.destinations[i].Enabled = true
			return true
		}
	}

	return false
}

// DisableDestination disables a destination without removing it
func (f *FlexLog) DisableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.Name == name {
			f.destinations[i].Enabled = false
			return true
		}
	}

	return false
}

// ListDestinations returns a copy of all configured destinations
func (f *FlexLog) ListDestinations() []LogDestination {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a copy to avoid race conditions
	destCopy := make([]LogDestination, len(f.destinations))
	copy(destCopy, f.destinations)

	return destCopy
}

// writeToDestinations writes data to all enabled destinations
func (f *FlexLog) writeToDestinations(p []byte) (int, error) {
	if len(f.destinations) == 0 {
		return len(p), nil // No destinations, pretend we wrote everything
	}

	var lastErr error
	var wg sync.WaitGroup

	// Create a copy of destinations to avoid holding the lock during writes
	f.mu.Lock()
	dests := make([]LogDestination, 0, len(f.destinations))
	for _, d := range f.destinations {
		if d.Enabled {
			dests = append(dests, d)
		}
	}
	f.mu.Unlock()

	// Write to each destination concurrently
	errChan := make(chan error, len(dests))
	for _, dest := range dests {
		wg.Add(1)
		go func(d LogDestination) {
			defer wg.Done()
			_, err := d.Writer.Write(p)
			if err != nil {
				errChan <- err
			}
		}(dest)
	}

	// Wait for all writes to complete
	wg.Wait()
	close(errChan)

	// Return the last error if any occurred
	for err := range errChan {
		lastErr = err
	}

	return len(p), lastErr
}
