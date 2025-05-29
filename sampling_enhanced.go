package flexlog

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AdaptiveSamplingConfig configures adaptive sampling behavior
type AdaptiveSamplingConfig struct {
	// Target logs per second
	TargetRate float64

	// Window duration for rate calculation
	WindowDuration time.Duration

	// Minimum sampling rate (never go below this)
	MinRate float64

	// Maximum sampling rate (never go above this)
	MaxRate float64

	// Adjustment factor for rate changes
	AdjustmentFactor float64

	// Enable level-based adjustments
	LevelAdjustments bool
}

// LevelSamplingConfig configures per-level sampling rates
type LevelSamplingConfig struct {
	TraceRate float64
	DebugRate float64
	InfoRate  float64
	WarnRate  float64
	ErrorRate float64
}

// PatternSamplingRule defines a sampling rule based on patterns
type PatternSamplingRule struct {
	// Pattern to match (regex or simple string)
	Pattern string

	// Rate for messages matching this pattern
	Rate float64

	// Whether this rule overrides global sampling
	Override bool

	// Priority (higher priority rules are evaluated first)
	Priority int
}

// SamplingMetrics tracks sampling statistics
type SamplingMetrics struct {
	TotalMessages   uint64
	SampledMessages uint64
	DroppedMessages uint64
	
	// Per-level metrics
	LevelMetrics map[int]*LevelSamplingMetrics
	
	// Pattern match counts
	PatternMatches map[string]uint64
	
	mu sync.RWMutex
}

// LevelSamplingMetrics tracks per-level sampling statistics
type LevelSamplingMetrics struct {
	Total   uint64
	Sampled uint64
	Dropped uint64
}

// AdaptiveSampler implements adaptive sampling based on rate
type AdaptiveSampler struct {
	config       AdaptiveSamplingConfig
	currentRate  float64
	messageCount uint64
	windowStart  time.Time
	mu           sync.RWMutex
}

// NewAdaptiveSampler creates a new adaptive sampler
func NewAdaptiveSampler(config AdaptiveSamplingConfig) *AdaptiveSampler {
	if config.WindowDuration == 0 {
		config.WindowDuration = 10 * time.Second
	}
	if config.MinRate == 0 {
		config.MinRate = 0.01 // 1%
	}
	if config.MaxRate == 0 {
		config.MaxRate = 1.0 // 100%
	}
	if config.AdjustmentFactor == 0 {
		config.AdjustmentFactor = 0.1
	}

	return &AdaptiveSampler{
		config:      config,
		currentRate: 1.0,
		windowStart: time.Now(),
	}
}

// ShouldSample determines if a message should be sampled
func (as *AdaptiveSampler) ShouldSample() bool {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Check if window has elapsed
	now := time.Now()
	elapsed := now.Sub(as.windowStart)
	if elapsed >= as.config.WindowDuration {
		// Calculate current rate
		currentRate := float64(as.messageCount) / elapsed.Seconds()
		
		// Adjust sampling rate
		if currentRate > as.config.TargetRate && as.currentRate > as.config.MinRate {
			// Too many messages, decrease sampling rate
			adjustment := math.Min(as.config.AdjustmentFactor, as.currentRate-as.config.MinRate)
			as.currentRate -= adjustment
		} else if currentRate < as.config.TargetRate*0.8 && as.currentRate < as.config.MaxRate {
			// Too few messages, increase sampling rate
			adjustment := math.Min(as.config.AdjustmentFactor, as.config.MaxRate-as.currentRate)
			as.currentRate += adjustment
		}

		// Reset window
		as.messageCount = 0
		as.windowStart = now
	}

	as.messageCount++
	return rand.Float64() < as.currentRate
}

// GetCurrentRate returns the current sampling rate
func (as *AdaptiveSampler) GetCurrentRate() float64 {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.currentRate
}

// SetupEnhancedSampling configures enhanced sampling features
func (f *FlexLog) SetupEnhancedSampling() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Initialize sampling metrics
	f.samplingMetrics = &SamplingMetrics{
		LevelMetrics:   make(map[int]*LevelSamplingMetrics),
		PatternMatches: make(map[string]uint64),
	}

	// Initialize per-level metrics
	for _, level := range []int{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError} {
		f.samplingMetrics.LevelMetrics[level] = &LevelSamplingMetrics{}
	}
}

// SetLevelSampling configures per-level sampling rates
func (f *FlexLog) SetLevelSampling(config LevelSamplingConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.levelSampling == nil {
		f.levelSampling = make(map[int]float64)
	}

	f.levelSampling[LevelTrace] = config.TraceRate
	f.levelSampling[LevelDebug] = config.DebugRate
	f.levelSampling[LevelInfo] = config.InfoRate
	f.levelSampling[LevelWarn] = config.WarnRate
	f.levelSampling[LevelError] = config.ErrorRate
}

// SetAdaptiveSampling enables adaptive sampling
func (f *FlexLog) SetAdaptiveSampling(config AdaptiveSamplingConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.adaptiveSampler = NewAdaptiveSampler(config)
	f.samplingStrategy = SamplingAdaptive
}

// AddPatternSamplingRule adds a pattern-based sampling rule
func (f *FlexLog) AddPatternSamplingRule(rule PatternSamplingRule) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.patternRules == nil {
		f.patternRules = make([]PatternSamplingRule, 0)
	}

	// Validate pattern
	if rule.Pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	if rule.Rate < 0 || rule.Rate > 1 {
		return fmt.Errorf("rate must be between 0 and 1")
	}

	f.patternRules = append(f.patternRules, rule)
	
	// Sort by priority (descending)
	sortPatternRules(f.patternRules)

	return nil
}

// shouldLogEnhanced is an enhanced version of shouldLog with additional sampling strategies
func (f *FlexLog) shouldLogEnhanced(level int, message string, fields map[string]interface{}) bool {
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

	// Track total messages
	atomic.AddUint64(&f.samplingMetrics.TotalMessages, 1)
	if metrics, ok := f.samplingMetrics.LevelMetrics[level]; ok {
		atomic.AddUint64(&metrics.Total, 1)
	}

	// Check pattern-based rules first
	sampled := f.checkPatternSampling(message, fields)
	if sampled != nil {
		if *sampled {
			f.recordSampled(level)
		} else {
			f.recordDropped(level)
		}
		return *sampled
	}

	// Check level-based sampling
	if f.levelSampling != nil {
		if rate, ok := f.levelSampling[level]; ok && rate < 1.0 {
			if rand.Float64() >= rate {
				f.recordDropped(level)
				return false
			}
		}
	}

	// Apply global sampling strategy
	shouldSample := false
	switch f.samplingStrategy {
	case SamplingNone:
		shouldSample = true

	case SamplingRandom:
		shouldSample = rand.Float64() < f.samplingRate

	case SamplingConsistent:
		if f.samplingRate >= 1.0 {
			shouldSample = true
		} else {
			key := f.sampleKeyFunc(level, message, fields)
			h := fnv.New32a()
			h.Write([]byte(key))
			hash := h.Sum32()
			shouldSample = float64(hash%1000)/1000.0 < f.samplingRate
		}

	case SamplingInterval:
		if f.samplingRate <= 1.0 {
			shouldSample = true
		} else {
			counter := atomic.AddUint64(&f.sampleCounter, 1)
			shouldSample = counter%uint64(f.samplingRate) == 1
		}

	case SamplingAdaptive:
		if f.adaptiveSampler != nil {
			shouldSample = f.adaptiveSampler.ShouldSample()
		} else {
			shouldSample = true
		}

	default:
		shouldSample = true
	}

	// Record metrics
	if shouldSample {
		f.recordSampled(level)
	} else {
		f.recordDropped(level)
	}

	return shouldSample
}

// checkPatternSampling checks if pattern-based sampling rules apply
func (f *FlexLog) checkPatternSampling(message string, fields map[string]interface{}) *bool {
	if f.patternRules == nil || len(f.patternRules) == 0 {
		return nil
	}

	for _, rule := range f.patternRules {
		// Simple string matching for now (could be extended to regex)
		if strings.Contains(message, rule.Pattern) {
			// Record pattern match
			f.samplingMetrics.mu.Lock()
			f.samplingMetrics.PatternMatches[rule.Pattern]++
			f.samplingMetrics.mu.Unlock()

			if rule.Override {
				// This rule overrides global sampling
				result := rand.Float64() < rule.Rate
				return &result
			}
		}
	}

	return nil
}

// recordSampled records that a message was sampled
func (f *FlexLog) recordSampled(level int) {
	atomic.AddUint64(&f.samplingMetrics.SampledMessages, 1)
	if metrics, ok := f.samplingMetrics.LevelMetrics[level]; ok {
		atomic.AddUint64(&metrics.Sampled, 1)
	}
}

// recordDropped records that a message was dropped
func (f *FlexLog) recordDropped(level int) {
	atomic.AddUint64(&f.samplingMetrics.DroppedMessages, 1)
	if metrics, ok := f.samplingMetrics.LevelMetrics[level]; ok {
		atomic.AddUint64(&metrics.Dropped, 1)
	}
}

// GetSamplingMetrics returns current sampling metrics
func (f *FlexLog) GetSamplingMetrics() SamplingMetrics {
	f.samplingMetrics.mu.RLock()
	defer f.samplingMetrics.mu.RUnlock()

	// Create a copy of the metrics
	metrics := SamplingMetrics{
		TotalMessages:   atomic.LoadUint64(&f.samplingMetrics.TotalMessages),
		SampledMessages: atomic.LoadUint64(&f.samplingMetrics.SampledMessages),
		DroppedMessages: atomic.LoadUint64(&f.samplingMetrics.DroppedMessages),
		LevelMetrics:    make(map[int]*LevelSamplingMetrics),
		PatternMatches:  make(map[string]uint64),
	}

	// Copy level metrics
	for level, levelMetrics := range f.samplingMetrics.LevelMetrics {
		metrics.LevelMetrics[level] = &LevelSamplingMetrics{
			Total:   atomic.LoadUint64(&levelMetrics.Total),
			Sampled: atomic.LoadUint64(&levelMetrics.Sampled),
			Dropped: atomic.LoadUint64(&levelMetrics.Dropped),
		}
	}

	// Copy pattern matches
	for pattern, count := range f.samplingMetrics.PatternMatches {
		metrics.PatternMatches[pattern] = count
	}

	return metrics
}

// ResetSamplingMetrics resets all sampling metrics
func (f *FlexLog) ResetSamplingMetrics() {
	f.samplingMetrics.mu.Lock()
	defer f.samplingMetrics.mu.Unlock()

	atomic.StoreUint64(&f.samplingMetrics.TotalMessages, 0)
	atomic.StoreUint64(&f.samplingMetrics.SampledMessages, 0)
	atomic.StoreUint64(&f.samplingMetrics.DroppedMessages, 0)

	for _, metrics := range f.samplingMetrics.LevelMetrics {
		atomic.StoreUint64(&metrics.Total, 0)
		atomic.StoreUint64(&metrics.Sampled, 0)
		atomic.StoreUint64(&metrics.Dropped, 0)
	}

	f.samplingMetrics.PatternMatches = make(map[string]uint64)
}

// sortPatternRules sorts pattern rules by priority (descending)
func sortPatternRules(rules []PatternSamplingRule) {
	// Simple bubble sort for small arrays
	n := len(rules)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if rules[j].Priority < rules[j+1].Priority {
				rules[j], rules[j+1] = rules[j+1], rules[j]
			}
		}
	}
}