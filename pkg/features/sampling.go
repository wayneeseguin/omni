package features

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SamplingStrategy defines how log sampling is performed to reduce log volume.
type SamplingStrategy int

const (
	// SamplingNone disables sampling - all messages are logged
	SamplingNone SamplingStrategy = iota
	// SamplingRandom randomly samples messages based on probability
	SamplingRandom
	// SamplingInterval logs every Nth message
	SamplingInterval
	// SamplingConsistent uses hash-based sampling for consistent decisions
	SamplingConsistent
	// SamplingAdaptive adjusts sampling rate based on traffic patterns
	SamplingAdaptive
	// SamplingRateLimited enforces a max messages per second limit
	SamplingRateLimited
	// SamplingBurst allows burst of messages then reduces rate
	SamplingBurst
)

// SamplingManager handles log sampling to reduce volume
type SamplingManager struct {
	mu              sync.RWMutex
	strategy        SamplingStrategy
	rate            float64
	counter         uint64
	keyFunc         func(level int, message string, fields map[string]interface{}) string
	adaptiveSampler *AdaptiveSampler
	levelSampling   map[int]float64
	patternRules    []PatternSamplingRule
	metrics         *SamplingMetrics
	errorHandler    func(source, dest, msg string, err error)
	metricsHandler  func(string)

	// Rate limiting fields
	rateLimiter  *RateLimiter
	maxPerSecond float64

	// Burst sampling fields
	burstWindow  time.Duration
	burstSize    int
	burstTracker *BurstTracker

	// Pattern caching
	compiledPatterns map[string]*regexp.Regexp
	patternsMu       sync.RWMutex
}

// AdaptiveSampler provides adaptive sampling functionality
type AdaptiveSampler struct {
	mu                sync.RWMutex
	windowSize        time.Duration
	targetRate        float64
	minRate           float64
	maxRate           float64
	currentRate       float64
	adjustInterval    time.Duration
	lastAdjust        time.Time
	messageCount      uint64
	windowStart       time.Time
	historyWindows    []AdaptiveWindow
	maxHistoryWindows int
}

// AdaptiveWindow tracks metrics for a time window
type AdaptiveWindow struct {
	Start        time.Time
	End          time.Time
	MessageCount uint64
	Rate         float64
}

// RateLimiter implements token bucket algorithm
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

// BurstTracker tracks burst patterns
type BurstTracker struct {
	mu            sync.Mutex
	windowStart   time.Time
	messageCount  int
	burstDetected bool
}

// PatternSamplingRule defines sampling rules based on message patterns
type PatternSamplingRule struct {
	Pattern     string
	Rate        float64
	Priority    int // Higher priority rules override lower ones
	Description string
	MatchFields bool // Whether to match against fields as well
}

// SamplingMetrics tracks sampling statistics
type SamplingMetrics struct {
	TotalMessages   uint64
	SampledMessages uint64
	DroppedMessages uint64
	CurrentRate     float64
	EffectiveRate   float64 // Actual sampling rate based on decisions
	StrategyHits    map[string]uint64
	LevelHits       map[int]uint64
	PatternHits     map[string]uint64
	LastUpdate      time.Time
}

// NewSamplingManager creates a new sampling manager
func NewSamplingManager() *SamplingManager {
	return &SamplingManager{
		strategy:         SamplingNone,
		rate:             1.0,
		keyFunc:          defaultSampleKeyFunc,
		compiledPatterns: make(map[string]*regexp.Regexp),
		levelSampling:    make(map[int]float64),
		metrics: &SamplingMetrics{
			CurrentRate:  1.0,
			StrategyHits: make(map[string]uint64),
			LevelHits:    make(map[int]uint64),
			PatternHits:  make(map[string]uint64),
			LastUpdate:   time.Now(),
		},
	}
}

// defaultSampleKeyFunc generates a default key for consistent sampling
func defaultSampleKeyFunc(level int, message string, fields map[string]interface{}) string {
	return message // Use the message as the key by default
}

// SetErrorHandler sets the error handling function
func (s *SamplingManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorHandler = handler
}

// SetMetricsHandler sets the metrics tracking function
func (s *SamplingManager) SetMetricsHandler(handler func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metricsHandler = handler
}

// SetStrategy sets the sampling strategy and rate
func (s *SamplingManager) SetStrategy(strategy SamplingStrategy, rate float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousStrategy := s.strategy
	s.strategy = strategy

	// Validate and normalize rate
	switch strategy {
	case SamplingNone:
		s.rate = 1.0

	case SamplingRandom, SamplingConsistent:
		// Ensure rate is between 0 and 1
		if rate < 0 {
			rate = 0
		} else if rate > 1 {
			rate = 1
		}
		s.rate = rate

	case SamplingInterval:
		// For interval, rate is the sampling interval
		if rate < 1 {
			rate = 1 // Log every message
		}
		s.rate = rate

	case SamplingAdaptive:
		// Initialize adaptive sampler
		if s.adaptiveSampler == nil {
			s.adaptiveSampler = NewAdaptiveSampler(rate, 0.01, 1.0)
		}
		s.rate = rate

	case SamplingRateLimited:
		// Initialize rate limiter
		s.maxPerSecond = rate
		s.rateLimiter = NewRateLimiter(rate)
		s.rate = rate

	case SamplingBurst:
		// Initialize burst tracker
		if s.burstWindow == 0 {
			s.burstWindow = time.Minute // Default 1 minute window
		}
		if s.burstSize == 0 {
			s.burstSize = int(rate) // Use rate as burst size
		}
		s.burstTracker = NewBurstTracker(s.burstWindow, s.burstSize)
		s.rate = rate

	default:
		return fmt.Errorf("unsupported sampling strategy: %v", strategy)
	}

	// Reset counter when changing sampling
	atomic.StoreUint64(&s.counter, 0)

	// Update metrics
	if s.metrics != nil {
		s.metrics.CurrentRate = s.rate
		s.metrics.LastUpdate = time.Now()
	}

	// Track strategy change
	if s.metricsHandler != nil && previousStrategy != strategy {
		s.metricsHandler(fmt.Sprintf("sampling_strategy_changed_%s", strategyName(strategy)))
	}

	return nil
}

// SetKeyFunc sets the function used to generate the key for consistent sampling
func (s *SamplingManager) SetKeyFunc(keyFunc func(level int, message string, fields map[string]interface{}) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if keyFunc != nil {
		s.keyFunc = keyFunc
	}
}

// SetLevelSampling sets sampling rates for specific log levels
func (s *SamplingManager) SetLevelSampling(levelRates map[int]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.levelSampling = levelRates
}

// SetPatternRules sets pattern-based sampling rules
func (s *SamplingManager) SetPatternRules(rules []PatternSamplingRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sort rules by priority (highest first)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	// Compile patterns
	s.patternsMu.Lock()
	defer s.patternsMu.Unlock()

	// Clear old patterns
	s.compiledPatterns = make(map[string]*regexp.Regexp)

	// Compile new patterns
	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			if s.errorHandler != nil {
				s.errorHandler("sampling", "", fmt.Sprintf("Invalid pattern: %s", rule.Pattern), err)
			}
			return fmt.Errorf("compile pattern %s: %w", rule.Pattern, err)
		}
		s.compiledPatterns[rule.Pattern] = re
	}

	s.patternRules = rules
	return nil
}

// SetBurstConfig sets burst sampling configuration
func (s *SamplingManager) SetBurstConfig(window time.Duration, burstSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.burstWindow = window
	s.burstSize = burstSize

	if s.strategy == SamplingBurst && s.burstTracker != nil {
		s.burstTracker = NewBurstTracker(window, burstSize)
	}
}

// SetAdaptiveConfig configures adaptive sampling parameters
func (s *SamplingManager) SetAdaptiveConfig(targetRate, minRate, maxRate float64, windowSize time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.adaptiveSampler == nil {
		s.adaptiveSampler = NewAdaptiveSampler(targetRate, minRate, maxRate)
	}

	s.adaptiveSampler.mu.Lock()
	s.adaptiveSampler.targetRate = targetRate
	s.adaptiveSampler.minRate = minRate
	s.adaptiveSampler.maxRate = maxRate
	s.adaptiveSampler.windowSize = windowSize
	s.adaptiveSampler.mu.Unlock()
}

// ShouldLog determines if a log entry should be logged based on sampling
func (s *SamplingManager) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Track total messages
	if s.metrics != nil {
		atomic.AddUint64(&s.metrics.TotalMessages, 1)
		s.trackLevelHit(level)
	}

	// Check pattern-based rules first (highest priority)
	if len(s.patternRules) > 0 {
		if decision, matched := s.checkPatternRules(message, fields); matched {
			s.updateMetrics(decision)
			return decision
		}
	}

	// Check level-specific sampling
	if s.levelSampling != nil {
		if rate, ok := s.levelSampling[level]; ok {
			// Use random sampling for level-specific rates
			randomVal, err := secureRandomFloat64()
			if err != nil {
				// On error, fall back to always log
				s.updateMetrics(true)
				return true
			}
			shouldLog := randomVal < rate
			s.updateMetrics(shouldLog)
			return shouldLog
		}
	}

	// Apply general sampling strategy
	shouldLog := s.applySampling(s.rate, level, message, fields)
	s.updateMetrics(shouldLog)
	return shouldLog
}

// secureRandomFloat64 generates a cryptographically secure random float64 in [0, 1)
func secureRandomFloat64() (float64, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	// Convert to uint64 and scale to [0, 1)
	// Use only 53 bits for precision (same as math/rand)
	uint64Val := binary.BigEndian.Uint64(b[:]) >> 11
	return float64(uint64Val) / float64(1<<53), nil
}

// checkPatternRules checks if any pattern rule matches
func (s *SamplingManager) checkPatternRules(message string, fields map[string]interface{}) (bool, bool) {
	s.patternsMu.RLock()
	defer s.patternsMu.RUnlock()

	for _, rule := range s.patternRules {
		if re, ok := s.compiledPatterns[rule.Pattern]; ok {
			// Check message
			if re.MatchString(message) {
				s.trackPatternHit(rule.Pattern)
				// Apply sampling rate for this pattern
				randomVal, err := secureRandomFloat64()
				if err != nil {
					// On error, fall back to always log
					return true, true
				}
				return randomVal < rule.Rate, true
			}

			// Check fields if enabled
			if rule.MatchFields && fields != nil {
				for _, v := range fields {
					if str, ok := v.(string); ok && re.MatchString(str) {
						s.trackPatternHit(rule.Pattern)
						randomVal, err := secureRandomFloat64()
						if err != nil {
							// On error, fall back to always log
							return true, true
						}
						return randomVal < rule.Rate, true
					}
				}
			}
		}
	}

	return false, false
}

// applySampling applies the configured sampling strategy
func (s *SamplingManager) applySampling(rate float64, level int, message string, fields map[string]interface{}) bool {
	s.trackStrategyHit(s.strategy)

	switch s.strategy {
	case SamplingNone:
		return true

	case SamplingRandom:
		randomVal, err := secureRandomFloat64()
		if err != nil {
			// On error, fall back to always log
			return true
		}
		return randomVal < rate

	case SamplingConsistent:
		if rate >= 1.0 {
			return true
		}

		// Use hash-based sampling for consistency
		key := s.keyFunc(level, message, fields)
		h := fnv.New32a()
		_, _ = h.Write([]byte(key)) // Hash write never fails
		hash := h.Sum32()
		return float64(hash%1000)/1000.0 < rate

	case SamplingInterval:
		if rate <= 1.0 {
			return true
		}

		counter := atomic.AddUint64(&s.counter, 1)
		return counter%uint64(rate) == 1

	case SamplingAdaptive:
		if s.adaptiveSampler != nil {
			return s.adaptiveSampler.ShouldLog(level, message, fields)
		}
		return true

	case SamplingRateLimited:
		if s.rateLimiter != nil {
			return s.rateLimiter.Allow()
		}
		return true

	case SamplingBurst:
		if s.burstTracker != nil {
			return s.burstTracker.ShouldLog()
		}
		return true
	}

	return true
}

// updateMetrics updates sampling metrics
func (s *SamplingManager) updateMetrics(shouldLog bool) {
	if s.metrics == nil {
		return
	}

	if shouldLog {
		atomic.AddUint64(&s.metrics.SampledMessages, 1)
	} else {
		atomic.AddUint64(&s.metrics.DroppedMessages, 1)
	}
}

// GetMetrics returns current sampling metrics
func (s *SamplingManager) GetMetrics() SamplingMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metrics == nil {
		return SamplingMetrics{}
	}

	metrics := SamplingMetrics{
		TotalMessages:   atomic.LoadUint64(&s.metrics.TotalMessages),
		SampledMessages: atomic.LoadUint64(&s.metrics.SampledMessages),
		DroppedMessages: atomic.LoadUint64(&s.metrics.DroppedMessages),
		CurrentRate:     s.metrics.CurrentRate,
		StrategyHits:    make(map[string]uint64),
		LevelHits:       make(map[int]uint64),
		PatternHits:     make(map[string]uint64),
	}

	// Copy maps
	for k, v := range s.metrics.StrategyHits {
		metrics.StrategyHits[k] = v
	}
	for k, v := range s.metrics.LevelHits {
		metrics.LevelHits[k] = v
	}
	for k, v := range s.metrics.PatternHits {
		metrics.PatternHits[k] = v
	}

	return metrics
}

// GetStrategy returns the current sampling strategy
func (s *SamplingManager) GetStrategy() SamplingStrategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.strategy
}

// GetRate returns the current sampling rate
func (s *SamplingManager) GetRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rate
}

// trackStrategyHit tracks strategy usage
func (s *SamplingManager) trackStrategyHit(strategy SamplingStrategy) {
	if s.metrics != nil && s.metrics.StrategyHits != nil {
		name := strategyName(strategy)
		// Use a simple counter approach to avoid map access races
		if current, ok := s.metrics.StrategyHits[name]; ok {
			s.metrics.StrategyHits[name] = current + 1
		} else {
			s.metrics.StrategyHits[name] = 1
		}
	}
}

// trackLevelHit tracks level usage
func (s *SamplingManager) trackLevelHit(level int) {
	if s.metrics != nil && s.metrics.LevelHits != nil {
		if current, ok := s.metrics.LevelHits[level]; ok {
			s.metrics.LevelHits[level] = current + 1
		} else {
			s.metrics.LevelHits[level] = 1
		}
	}
}

// trackPatternHit tracks pattern matches
func (s *SamplingManager) trackPatternHit(pattern string) {
	if s.metrics != nil && s.metrics.PatternHits != nil {
		if current, ok := s.metrics.PatternHits[pattern]; ok {
			s.metrics.PatternHits[pattern] = current + 1
		} else {
			s.metrics.PatternHits[pattern] = 1
		}
	}
}

// strategyName returns the name of a sampling strategy
func strategyName(strategy SamplingStrategy) string {
	switch strategy {
	case SamplingNone:
		return "none"
	case SamplingRandom:
		return "random"
	case SamplingInterval:
		return "interval"
	case SamplingConsistent:
		return "consistent"
	case SamplingAdaptive:
		return "adaptive"
	case SamplingRateLimited:
		return "rate_limited"
	case SamplingBurst:
		return "burst"
	default:
		return "unknown"
	}
}

// NewAdaptiveSampler creates a new adaptive sampler
func NewAdaptiveSampler(targetRate, minRate, maxRate float64) *AdaptiveSampler {
	return &AdaptiveSampler{
		targetRate:        targetRate,
		minRate:           minRate,
		maxRate:           maxRate,
		currentRate:       targetRate,
		windowSize:        time.Minute,
		adjustInterval:    time.Second * 10,
		lastAdjust:        time.Now(),
		windowStart:       time.Now(),
		maxHistoryWindows: 10,
		historyWindows:    make([]AdaptiveWindow, 0, 10),
	}
}

// ShouldLog determines if a message should be logged based on adaptive sampling
func (a *AdaptiveSampler) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Track message
	a.messageCount++

	// Check if we need to adjust the rate
	now := time.Now()
	if now.Sub(a.lastAdjust) >= a.adjustInterval {
		a.adjustRate(now)
		a.lastAdjust = now
	}

	// Apply current rate
	randomVal, err := secureRandomFloat64()
	if err != nil {
		// On error, fall back to always log
		return true
	}
	return randomVal < a.currentRate
}

// adjustRate adjusts the sampling rate based on traffic patterns
func (a *AdaptiveSampler) adjustRate(now time.Time) {
	// Calculate current window rate
	windowDuration := now.Sub(a.windowStart)
	if windowDuration >= a.windowSize {
		// Store current window
		window := AdaptiveWindow{
			Start:        a.windowStart,
			End:          now,
			MessageCount: a.messageCount,
			Rate:         a.currentRate,
		}

		a.historyWindows = append(a.historyWindows, window)
		if len(a.historyWindows) > a.maxHistoryWindows {
			a.historyWindows = a.historyWindows[1:]
		}

		// Reset for new window
		a.windowStart = now
		a.messageCount = 0
	}

	// Calculate message rate
	messagesPerSecond := float64(a.messageCount) / windowDuration.Seconds()

	// Adjust rate based on traffic
	if messagesPerSecond > 1000 { // High traffic
		a.currentRate = math.Max(a.minRate, a.currentRate*0.9)
	} else if messagesPerSecond < 100 { // Low traffic
		a.currentRate = math.Min(a.maxRate, a.currentRate*1.1)
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerSecond float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxPerSecond,
		maxTokens:  maxPerSecond,
		refillRate: maxPerSecond,
		lastRefill: time.Now(),
	}
}

// Allow checks if a message is allowed based on rate limiting
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	tokensToAdd := elapsed * r.refillRate

	r.tokens = math.Min(r.maxTokens, r.tokens+tokensToAdd)
	r.lastRefill = now

	// Check if we have tokens
	if r.tokens >= 1.0 {
		r.tokens--
		return true
	}

	return false
}

// NewBurstTracker creates a new burst tracker
func NewBurstTracker(window time.Duration, burstSize int) *BurstTracker {
	return &BurstTracker{
		windowStart: time.Now(),
	}
}

// ShouldLog determines if a message should be logged based on burst detection
func (b *BurstTracker) ShouldLog() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// Reset window if needed
	if now.Sub(b.windowStart) >= time.Minute { // Simplified to 1 minute windows
		b.windowStart = now
		b.messageCount = 0
		b.burstDetected = false
	}

	b.messageCount++

	// Detect burst
	if b.messageCount > 1000 && !b.burstDetected { // Threshold for burst detection
		b.burstDetected = true
	}

	// Apply burst sampling
	if b.burstDetected {
		// Sample more aggressively during burst
		return b.messageCount%10 == 0 // Log every 10th message
	}

	return true // Log all messages when not in burst
}

// Reset resets the sampling manager to default state
func (s *SamplingManager) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.strategy = SamplingNone
	s.rate = 1.0
	atomic.StoreUint64(&s.counter, 0)

	// Reset metrics
	if s.metrics != nil {
		s.metrics = &SamplingMetrics{
			CurrentRate:  1.0,
			StrategyHits: make(map[string]uint64),
			LevelHits:    make(map[int]uint64),
			PatternHits:  make(map[string]uint64),
			LastUpdate:   time.Now(),
		}
	}
}

// GetStatus returns the current status of the sampling manager
func (s *SamplingManager) GetStatus() SamplingStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := SamplingStatus{
		Strategy:     s.strategy,
		Rate:         s.rate,
		IsActive:     s.strategy != SamplingNone,
		LevelRates:   make(map[int]float64),
		PatternRules: make([]PatternSamplingRule, len(s.patternRules)),
	}

	// Copy level rates
	for level, rate := range s.levelSampling {
		status.LevelRates[level] = rate
	}

	// Copy pattern rules
	copy(status.PatternRules, s.patternRules)

	// Get adaptive status
	if s.adaptiveSampler != nil {
		s.adaptiveSampler.mu.RLock()
		status.AdaptiveRate = s.adaptiveSampler.currentRate
		s.adaptiveSampler.mu.RUnlock()
	}

	// Get rate limiter status
	if s.rateLimiter != nil {
		s.rateLimiter.mu.Lock()
		status.RateLimitTokens = s.rateLimiter.tokens
		s.rateLimiter.mu.Unlock()
	}

	return status
}

// SamplingStatus represents the current status of sampling
type SamplingStatus struct {
	Strategy        SamplingStrategy      `json:"strategy"`
	Rate            float64               `json:"rate"`
	IsActive        bool                  `json:"is_active"`
	LevelRates      map[int]float64       `json:"level_rates,omitempty"`
	PatternRules    []PatternSamplingRule `json:"pattern_rules,omitempty"`
	AdaptiveRate    float64               `json:"adaptive_rate,omitempty"`
	RateLimitTokens float64               `json:"rate_limit_tokens,omitempty"`
}

// ExportMetrics exports detailed metrics for analysis
func (s *SamplingManager) ExportMetrics() SamplingMetricsExport {
	metrics := s.GetMetrics()

	export := SamplingMetricsExport{
		TotalMessages:   metrics.TotalMessages,
		SampledMessages: metrics.SampledMessages,
		DroppedMessages: metrics.DroppedMessages,
		CurrentRate:     metrics.CurrentRate,
		EffectiveRate:   0,
		LastUpdate:      metrics.LastUpdate,
		Strategies:      make(map[string]uint64),
		Levels:          make(map[string]uint64),
		Patterns:        make(map[string]uint64),
	}

	// Calculate effective rate
	if metrics.TotalMessages > 0 {
		export.EffectiveRate = float64(metrics.SampledMessages) / float64(metrics.TotalMessages)
	}

	// Copy strategy hits
	for strategy, count := range metrics.StrategyHits {
		export.Strategies[strategy] = count
	}

	// Convert level hits to string keys for JSON
	for level, count := range metrics.LevelHits {
		export.Levels[fmt.Sprintf("level_%d", level)] = count
	}

	// Copy pattern hits
	for pattern, count := range metrics.PatternHits {
		export.Patterns[pattern] = count
	}

	return export
}

// SamplingMetricsExport represents exportable metrics
type SamplingMetricsExport struct {
	TotalMessages   uint64            `json:"total_messages"`
	SampledMessages uint64            `json:"sampled_messages"`
	DroppedMessages uint64            `json:"dropped_messages"`
	CurrentRate     float64           `json:"current_rate"`
	EffectiveRate   float64           `json:"effective_rate"`
	LastUpdate      time.Time         `json:"last_update"`
	Strategies      map[string]uint64 `json:"strategies"`
	Levels          map[string]uint64 `json:"levels"`
	Patterns        map[string]uint64 `json:"patterns"`
}
