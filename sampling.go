package flexlog

import (
	"hash/fnv"
	"math/rand"
	"sync/atomic"
)

// defaultSampleKeyFunc generates a default key for consistent sampling
func defaultSampleKeyFunc(level int, message string, fields map[string]interface{}) string {
	return message // Use the message as the key by default
}

// SetSampling configures log sampling
func (f *FlexLog) SetSampling(strategy SamplingStrategy, rate float64) {
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

// SetSampleKeyFunc sets the function used to generate the key for consistent sampling
func (f *FlexLog) SetSampleKeyFunc(keyFunc func(level int, message string, fields map[string]interface{}) string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if keyFunc != nil {
		f.sampleKeyFunc = keyFunc
	}
}

// shouldLog determines if a log entry should be logged based on filters and sampling
func (f *FlexLog) shouldLog(level int, message string, fields map[string]interface{}) bool {
	// Quick check for log level
	if level < f.level {
		return false
	}

	// Apply filters
	if len(f.filters) > 0 {
		pass := false
		for _, filter := range f.filters {
			if filter(level, message, fields) {
				pass = true
				break
			}
		}
		if !pass {
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
