package flexlog

import (
	"hash/fnv"
	"math/rand"
	"sync/atomic"
)

// defaultSampleKeyFunc generates a default key for consistent sampling.
// By default, it uses the message content as the key, ensuring identical
// messages are consistently sampled the same way.
//
// Parameters:
//   - level: The log level
//   - message: The log message
//   - fields: Structured fields (if any)
//
// Returns:
//   - string: The key used for sampling decisions
func defaultSampleKeyFunc(level int, message string, fields map[string]interface{}) string {
	return message // Use the message as the key by default
}


// SetSampleKeyFunc sets the function used to generate the key for consistent sampling.
// This allows customization of how messages are grouped for sampling decisions.
//
// Parameters:
//   - keyFunc: Function that generates a key from log entry components
//
// Example:
//
//	logger.SetSampleKeyFunc(func(level int, msg string, fields map[string]interface{}) string {
//	    // Sample based on user ID to ensure consistent sampling per user
//	    if userID, ok := fields["user_id"].(string); ok {
//	        return userID
//	    }
//	    return msg
//	})
func (f *FlexLog) SetSampleKeyFunc(keyFunc func(level int, message string, fields map[string]interface{}) string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if keyFunc != nil {
		f.sampleKeyFunc = keyFunc
	}
}

// shouldLog determines if a log entry should be logged based on filters and sampling.
// It applies level checks, filters, and sampling strategies in that order.
// This is an internal method used by the logging functions.
//
// Parameters:
//   - level: The log level
//   - message: The log message
//   - fields: Structured fields (if any)
//
// Returns:
//   - bool: true if the message should be logged, false otherwise
func (f *FlexLog) shouldLog(level int, message string, fields map[string]interface{}) bool {
	// Use enhanced sampling if metrics are initialized
	if f.samplingMetrics != nil {
		return f.shouldLogEnhanced(level, message, fields)
	}
	// Quick check for log level
	if level < f.level {
		return false
	}

	// Apply filters - all filters must pass (AND logic)
	for _, filter := range f.filters {
		if !filter(level, message, fields) {
			return false
		}
	}

	// Apply sampling
	switch SamplingStrategy(f.samplingStrategy) {
	case SamplingNone:
		return true

	case SamplingRandom:
		return rand.Float64() < f.samplingRate

	case SamplingConsistent:
		if f.samplingRate >= 1.0 {
			return true
		}

		// Use hash-based sampling for consistency
		key := f.sampleKeyFunc(level, message, fields)
		h := fnv.New32a()
		h.Write([]byte(key))
		hash := h.Sum32()
		return float64(hash%1000)/1000.0 < f.samplingRate

	case SamplingInterval:
		if f.samplingRate <= 1.0 {
			return true
		}

		counter := atomic.AddUint64(&f.sampleCounter, 1)
		return counter%uint64(f.samplingRate) == 1
	}

	return true
}

// GetSamplingRate returns the current sampling rate.
// For random sampling, this is a probability between 0 and 1.
// For interval sampling, this is the interval N (log every Nth message).
//
// Returns:
//   - float64: The current sampling rate
func (f *FlexLog) GetSamplingRate() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.samplingRate
}

// SetSampling configures log sampling (implements SamplableLogger interface).
// This is a convenience method that wraps SetSamplingStrategy.
//
// Parameters:
//   - strategy: The sampling strategy (as int)
//   - rate: The sampling rate
//
// Returns:
//   - error: Always returns nil (kept for interface compatibility)
func (f *FlexLog) SetSampling(strategy int, rate float64) error {
	f.SetSamplingStrategy(SamplingStrategy(strategy), rate)
	return nil
}

// SetSamplingStrategy configures log sampling with a typed strategy.
// Sampling reduces log volume by only logging a subset of messages.
//
// Parameters:
//   - strategy: The sampling strategy to use
//   - rate: The sampling rate (meaning depends on strategy)
//
// Strategy-specific rate meanings:
//   - SamplingNone: rate is ignored, all messages logged
//   - SamplingRandom: rate is probability (0.0 to 1.0)
//   - SamplingInterval: rate is N, log every Nth message
//   - SamplingConsistent: rate is probability for hash-based sampling
//
// Example:
//
//	logger.SetSamplingStrategy(flexlog.SamplingRandom, 0.1)     // Log 10% randomly
//	logger.SetSamplingStrategy(flexlog.SamplingInterval, 100)   // Log every 100th message
func (f *FlexLog) SetSamplingStrategy(strategy SamplingStrategy, rate float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.samplingStrategy = int(strategy)

	// Validate and normalize rate
	switch strategy {
	case SamplingRandom, SamplingConsistent:
		// Ensure rate is between 0 and 1
		if rate < 0 {
			rate = 0
		} else if rate > 1 {
			rate = 1
		}
	case SamplingInterval:
		// For interval, rate is the sampling interval
		if rate < 1 {
			rate = 1 // Log every message
		}
	}

	f.samplingRate = rate
	atomic.StoreUint64(&f.sampleCounter, 0) // Reset counter when changing sampling
}

