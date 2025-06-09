package features

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNilFilter is returned when a nil filter is passed
var ErrNilFilter = errors.New("filter cannot be nil")

// FilterFunc is a function that determines if a log entry should be logged.
// It receives the log level, message, and structured fields.
// Returns true if the message should be logged, false to filter it out.
type FilterFunc func(level int, message string, fields map[string]interface{}) bool

// FilterManager handles log filtering
type FilterManager struct {
	mu             sync.RWMutex
	filters        []NamedFilter
	chains         map[string]*FilterChain
	errorHandler   func(source, dest, msg string, err error)
	metricsHandler func(string)
	metrics        *FilterMetrics
	cacheEnabled   bool
	cache          *FilterCache
}

// NamedFilter represents a filter with metadata
type NamedFilter struct {
	Name        string
	Description string
	Filter      FilterFunc
	Priority    int  // Higher priority filters are evaluated first
	Enabled     bool // Can be toggled without removing
	Tags        []string
}

// FilterChain represents a chain of filters with specific behavior
type FilterChain struct {
	Name        string
	Mode        ChainMode // AND, OR, XOR modes
	Filters     []NamedFilter
	StopOnMatch bool // Stop evaluating after first match (for OR mode)
	Inverted    bool // Invert the final result
}

// ChainMode defines how filters in a chain are combined
type ChainMode int

const (
	// ChainModeAND requires all filters to pass
	ChainModeAND ChainMode = iota
	// ChainModeOR requires at least one filter to pass
	ChainModeOR
	// ChainModeXOR requires exactly one filter to pass
	ChainModeXOR
)

// FilterCache provides caching for filter decisions
type FilterCache struct {
	mu        sync.RWMutex
	cache     map[string]filterCacheEntry
	maxSize   int
	ttl       time.Duration
	hits      uint64
	misses    uint64
	evictions uint64
}

type filterCacheEntry struct {
	decision  bool
	timestamp time.Time
}

// FilterMetrics tracks filtering statistics
type FilterMetrics struct {
	TotalChecks      uint64
	TotalPassed      uint64
	TotalFiltered    uint64
	FilterHits       map[string]uint64
	ChainHits        map[string]uint64
	CacheHits        uint64
	CacheMisses      uint64
	ProcessingTimeNs int64
	LastUpdate       time.Time
}

// NewFilterManager creates a new filter manager
func NewFilterManager() *FilterManager {
	return &FilterManager{
		filters: make([]NamedFilter, 0),
		chains:  make(map[string]*FilterChain),
		metrics: &FilterMetrics{
			FilterHits: make(map[string]uint64),
			ChainHits:  make(map[string]uint64),
			LastUpdate: time.Now(),
		},
	}
}

// SetErrorHandler sets the error handling function
func (f *FilterManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errorHandler = handler
}

// SetMetricsHandler sets the metrics tracking function
func (f *FilterManager) SetMetricsHandler(handler func(string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsHandler = handler
}

// EnableCache enables caching of filter decisions
func (f *FilterManager) EnableCache(maxSize int, ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cacheEnabled = true
	f.cache = &FilterCache{
		cache:   make(map[string]filterCacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	if f.metricsHandler != nil {
		f.metricsHandler("filter_cache_enabled")
	}
}

// DisableCache disables filter decision caching
func (f *FilterManager) DisableCache() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cacheEnabled = false
	f.cache = nil

	if f.metricsHandler != nil {
		f.metricsHandler("filter_cache_disabled")
	}
}

// AddFilter adds a filter function that determines whether a log entry should be logged.
// Filters are applied in the order they were added. A message is logged only if all
// filters return true.
func (f *FilterManager) AddFilter(filter FilterFunc) error {
	return f.AddNamedFilter("", "", filter, 0)
}

// AddNamedFilter adds a named filter with metadata
func (f *FilterManager) AddNamedFilter(name, description string, filter FilterFunc, priority int) error {
	if filter == nil {
		return ErrNilFilter
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate name if not provided
	if name == "" {
		name = fmt.Sprintf("filter_%d", len(f.filters))
	}

	namedFilter := NamedFilter{
		Name:        name,
		Description: description,
		Filter:      filter,
		Priority:    priority,
		Enabled:     true,
	}

	// Insert filter in priority order
	inserted := false
	for i, existing := range f.filters {
		if priority > existing.Priority {
			f.filters = append(f.filters[:i], append([]NamedFilter{namedFilter}, f.filters[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		f.filters = append(f.filters, namedFilter)
	}

	if f.metricsHandler != nil {
		f.metricsHandler(fmt.Sprintf("filter_added_%s", name))
	}

	return nil
}

// RemoveFilter removes a filter by name
func (f *FilterManager) RemoveFilter(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, filter := range f.filters {
		if filter.Name == name {
			f.filters = append(f.filters[:i], f.filters[i+1:]...)
			if f.metricsHandler != nil {
				f.metricsHandler(fmt.Sprintf("filter_removed_%s", name))
			}
			return nil
		}
	}

	return fmt.Errorf("filter not found: %s", name)
}

// EnableFilter enables a filter by name
func (f *FilterManager) EnableFilter(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, filter := range f.filters {
		if filter.Name == name {
			f.filters[i].Enabled = true
			return nil
		}
	}

	return fmt.Errorf("filter not found: %s", name)
}

// DisableFilter disables a filter by name without removing it
func (f *FilterManager) DisableFilter(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, filter := range f.filters {
		if filter.Name == name {
			f.filters[i].Enabled = false
			return nil
		}
	}

	return fmt.Errorf("filter not found: %s", name)
}

// AddFilterChain adds a chain of filters with specific combination logic
func (f *FilterManager) AddFilterChain(chain *FilterChain) error {
	if chain == nil {
		return fmt.Errorf("chain cannot be nil")
	}

	if chain.Name == "" {
		return fmt.Errorf("chain name cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.chains[chain.Name] = chain

	if f.metricsHandler != nil {
		f.metricsHandler(fmt.Sprintf("filter_chain_added_%s", chain.Name))
	}

	return nil
}

// RemoveFilterChain removes a filter chain by name
func (f *FilterManager) RemoveFilterChain(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.chains[name]; !exists {
		return fmt.Errorf("chain not found: %s", name)
	}

	delete(f.chains, name)

	if f.metricsHandler != nil {
		f.metricsHandler(fmt.Sprintf("filter_chain_removed_%s", name))
	}

	return nil
}

// ClearFilters removes all filters.
// After calling this, all messages will be logged (subject to level and sampling).
func (f *FilterManager) ClearFilters() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = nil
	f.chains = make(map[string]*FilterChain)

	// Clear cache
	if f.cache != nil {
		f.cache.Clear()
	}

	if f.metricsHandler != nil {
		f.metricsHandler("filters_cleared")
	}
}

// ApplyFilters checks if a message should be logged based on all filters.
// Returns true if all filters pass, false if any filter rejects the message.
func (f *FilterManager) ApplyFilters(level int, message string, fields map[string]interface{}) bool {
	start := time.Now()
	defer func() {
		if f.metrics != nil {
			elapsed := time.Since(start).Nanoseconds()
			atomic.AddInt64(&f.metrics.ProcessingTimeNs, elapsed)
		}
	}()

	// Track total checks
	if f.metrics != nil {
		atomic.AddUint64(&f.metrics.TotalChecks, 1)
	}

	// Check cache first if enabled
	if f.cacheEnabled && f.cache != nil {
		cacheKey := f.generateCacheKey(level, message, fields)
		if decision, hit := f.cache.Get(cacheKey); hit {
			f.updateMetrics(decision, true)
			return decision
		}
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Apply individual filters first (AND logic by default)
	for _, filter := range f.filters {
		if !filter.Enabled {
			continue
		}

		decision := filter.Filter(level, message, fields)
		if f.metrics != nil && f.metrics.FilterHits != nil {
			f.metrics.FilterHits[filter.Name]++
		}

		if !decision {
			f.cacheDecision(level, message, fields, false)
			f.updateMetrics(false, false)
			return false
		}
	}

	// Apply filter chains
	for _, chain := range f.chains {
		decision := f.applyChain(chain, level, message, fields)
		if !decision {
			f.cacheDecision(level, message, fields, false)
			f.updateMetrics(false, false)
			return false
		}
	}

	// All filters passed
	f.cacheDecision(level, message, fields, true)
	f.updateMetrics(true, false)
	return true
}

// applyChain applies a filter chain with its specific logic
func (f *FilterManager) applyChain(chain *FilterChain, level int, message string, fields map[string]interface{}) bool {
	if chain == nil || len(chain.Filters) == 0 {
		return true
	}

	// Track chain usage
	if f.metrics != nil && f.metrics.ChainHits != nil {
		f.metrics.ChainHits[chain.Name]++
	}

	var result bool
	switch chain.Mode {
	case ChainModeAND:
		result = f.applyChainAND(chain, level, message, fields)
	case ChainModeOR:
		result = f.applyChainOR(chain, level, message, fields)
	case ChainModeXOR:
		result = f.applyChainXOR(chain, level, message, fields)
	default:
		result = true
	}

	// Apply inversion if needed
	if chain.Inverted {
		result = !result
	}

	return result
}

// applyChainAND applies AND logic to filter chain
func (f *FilterManager) applyChainAND(chain *FilterChain, level int, message string, fields map[string]interface{}) bool {
	for _, filter := range chain.Filters {
		if !filter.Enabled {
			continue
		}

		if !filter.Filter(level, message, fields) {
			return false
		}
	}
	return true
}

// applyChainOR applies OR logic to filter chain
func (f *FilterManager) applyChainOR(chain *FilterChain, level int, message string, fields map[string]interface{}) bool {
	for _, filter := range chain.Filters {
		if !filter.Enabled {
			continue
		}

		if filter.Filter(level, message, fields) {
			if chain.StopOnMatch {
				return true
			}
		}
	}
	return false
}

// applyChainXOR applies XOR logic to filter chain
func (f *FilterManager) applyChainXOR(chain *FilterChain, level int, message string, fields map[string]interface{}) bool {
	matches := 0
	for _, filter := range chain.Filters {
		if !filter.Enabled {
			continue
		}

		if filter.Filter(level, message, fields) {
			matches++
			if matches > 1 {
				return false // More than one match
			}
		}
	}
	return matches == 1
}

// generateCacheKey generates a cache key for filter decisions
func (f *FilterManager) generateCacheKey(level int, message string, fields map[string]interface{}) string {
	// Simple key generation - can be improved
	key := fmt.Sprintf("%d:%s", level, message)
	if len(fields) > 0 {
		// Add first few field keys to cache key
		fieldKeys := make([]string, 0, 3)
		for k := range fields {
			fieldKeys = append(fieldKeys, k)
			if len(fieldKeys) >= 3 {
				break
			}
		}
		key += ":" + strings.Join(fieldKeys, ",")
	}
	return key
}

// cacheDecision caches a filter decision
func (f *FilterManager) cacheDecision(level int, message string, fields map[string]interface{}, decision bool) {
	if !f.cacheEnabled || f.cache == nil {
		return
	}

	key := f.generateCacheKey(level, message, fields)
	f.cache.Set(key, decision)
}

// updateMetrics updates filter metrics
func (f *FilterManager) updateMetrics(passed bool, fromCache bool) {
	if f.metrics == nil {
		return
	}

	if passed {
		atomic.AddUint64(&f.metrics.TotalPassed, 1)
	} else {
		atomic.AddUint64(&f.metrics.TotalFiltered, 1)
	}

	if fromCache {
		atomic.AddUint64(&f.metrics.CacheHits, 1)
	} else {
		atomic.AddUint64(&f.metrics.CacheMisses, 1)
	}

	f.metrics.LastUpdate = time.Now()
}

// CreateFieldFilter creates a filter that only logs entries containing specific field values.
// The filter checks if the specified field exists and matches any of the provided values.
func CreateFieldFilter(field string, values ...interface{}) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}

		val, exists := fields[field]
		if !exists {
			return false
		}

		for _, v := range values {
			if val == v {
				return true
			}
		}
		return false
	}
}

// CreateLevelFieldFilter creates a filter that only logs entries with a specific level and field value.
// This allows fine-grained control over what gets logged based on both level and context.
func CreateLevelFieldFilter(logLevel int, field string, value interface{}) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if level != logLevel {
			return false
		}

		if fields == nil {
			return false
		}

		val, exists := fields[field]
		return exists && val == value
	}
}

// CreateRegexFilter creates a filter that only logs entries matching a regex pattern.
// Messages that don't match the pattern will be filtered out.
func CreateRegexFilter(pattern *regexp.Regexp) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		return pattern.MatchString(message)
	}
}

// CreateExcludeRegexFilter creates a filter that excludes entries matching a regex pattern.
// Messages that match the pattern will be filtered out (opposite of CreateRegexFilter).
func CreateExcludeRegexFilter(pattern *regexp.Regexp) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		return !pattern.MatchString(message)
	}
}

// CreateLevelFilter creates a filter that only logs messages at or above a certain level.
func CreateLevelFilter(minLevel int) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		return level >= minLevel
	}
}

// CreateFieldExistsFilter creates a filter that only logs entries containing a specific field.
func CreateFieldExistsFilter(field string) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}
		_, exists := fields[field]
		return exists
	}
}

// CreateFieldNotExistsFilter creates a filter that only logs entries NOT containing a specific field.
func CreateFieldNotExistsFilter(field string) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return true
		}
		_, exists := fields[field]
		return !exists
	}
}

// CreateMultiFieldFilter creates a filter that requires all specified fields to exist.
func CreateMultiFieldFilter(fields ...string) FilterFunc {
	return func(level int, message string, logFields map[string]interface{}) bool {
		if logFields == nil {
			return false
		}
		for _, field := range fields {
			if _, exists := logFields[field]; !exists {
				return false
			}
		}
		return true
	}
}

// GetFilterCount returns the number of active filters
func (f *FilterManager) GetFilterCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	count := 0
	for _, filter := range f.filters {
		if filter.Enabled {
			count++
		}
	}

	// Add chain filters
	for _, chain := range f.chains {
		for _, filter := range chain.Filters {
			if filter.Enabled {
				count++
			}
		}
	}

	return count
}

// Get returns a filter by name
func (f *FilterManager) Get(name string) (*NamedFilter, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, filter := range f.filters {
		if filter.Name == name {
			return &filter, true
		}
	}

	return nil, false
}

// List returns all filters
func (f *FilterManager) List() []NamedFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]NamedFilter, len(f.filters))
	copy(result, f.filters)
	return result
}

// ListChains returns all filter chains
func (f *FilterManager) ListChains() map[string]*FilterChain {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]*FilterChain)
	for k, v := range f.chains {
		result[k] = v
	}
	return result
}

// GetMetrics returns current filter metrics
func (f *FilterManager) GetMetrics() FilterMetrics {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.metrics == nil {
		return FilterMetrics{}
	}

	// Create a copy
	metrics := FilterMetrics{
		TotalChecks:      atomic.LoadUint64(&f.metrics.TotalChecks),
		TotalPassed:      atomic.LoadUint64(&f.metrics.TotalPassed),
		TotalFiltered:    atomic.LoadUint64(&f.metrics.TotalFiltered),
		CacheHits:        atomic.LoadUint64(&f.metrics.CacheHits),
		CacheMisses:      atomic.LoadUint64(&f.metrics.CacheMisses),
		ProcessingTimeNs: atomic.LoadInt64(&f.metrics.ProcessingTimeNs),
		LastUpdate:       f.metrics.LastUpdate,
		FilterHits:       make(map[string]uint64),
		ChainHits:        make(map[string]uint64),
	}

	// Copy filter hits
	for k, v := range f.metrics.FilterHits {
		metrics.FilterHits[k] = v
	}

	// Copy chain hits
	for k, v := range f.metrics.ChainHits {
		metrics.ChainHits[k] = v
	}

	return metrics
}

// ResetMetrics resets all filter metrics
func (f *FilterManager) ResetMetrics() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.metrics = &FilterMetrics{
		FilterHits: make(map[string]uint64),
		ChainHits:  make(map[string]uint64),
		LastUpdate: time.Now(),
	}
}

// GetStatus returns the current status of the filter manager
func (f *FilterManager) GetStatus() FilterStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	status := FilterStatus{
		FilterCount:     len(f.filters),
		ChainCount:      len(f.chains),
		CacheEnabled:    f.cacheEnabled,
		ActiveFilters:   0,
		DisabledFilters: 0,
	}

	// Count active/disabled filters
	for _, filter := range f.filters {
		if filter.Enabled {
			status.ActiveFilters++
		} else {
			status.DisabledFilters++
		}
	}

	// Add cache status
	if f.cache != nil {
		status.CacheSize = len(f.cache.cache)
		status.CacheMaxSize = f.cache.maxSize
		status.CacheTTL = f.cache.ttl
	}

	return status
}

// FilterStatus represents the current status of filtering
type FilterStatus struct {
	FilterCount     int           `json:"filter_count"`
	ChainCount      int           `json:"chain_count"`
	ActiveFilters   int           `json:"active_filters"`
	DisabledFilters int           `json:"disabled_filters"`
	CacheEnabled    bool          `json:"cache_enabled"`
	CacheSize       int           `json:"cache_size,omitempty"`
	CacheMaxSize    int           `json:"cache_max_size,omitempty"`
	CacheTTL        time.Duration `json:"cache_ttl,omitempty"`
}

// Cache methods

// Get retrieves a cached filter decision
func (c *FilterCache) Get(key string) (bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		atomic.AddUint64(&c.misses, 1)
		return false, false
	}

	// Check TTL
	if time.Since(entry.timestamp) > c.ttl {
		// Expired
		atomic.AddUint64(&c.misses, 1)
		return false, false
	}

	atomic.AddUint64(&c.hits, 1)
	return entry.decision, true
}

// Set stores a filter decision in cache
func (c *FilterCache) Set(key string, decision bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check size limit
	if len(c.cache) >= c.maxSize {
		// Simple eviction - remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		delete(c.cache, oldestKey)
		atomic.AddUint64(&c.evictions, 1)
	}

	c.cache[key] = filterCacheEntry{
		decision:  decision,
		timestamp: time.Now(),
	}
}

// Clear removes all entries from cache
func (c *FilterCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]filterCacheEntry)
	atomic.StoreUint64(&c.hits, 0)
	atomic.StoreUint64(&c.misses, 0)
	atomic.StoreUint64(&c.evictions, 0)
}

// GetStats returns cache statistics
func (c *FilterCache) GetStats() (hits, misses, evictions uint64) {
	return atomic.LoadUint64(&c.hits),
		atomic.LoadUint64(&c.misses),
		atomic.LoadUint64(&c.evictions)
}

// Additional filter creation helpers

// CreateCompositeFilter creates a filter that combines multiple filters with AND logic
func CreateCompositeFilter(filters ...FilterFunc) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		for _, filter := range filters {
			if !filter(level, message, fields) {
				return false
			}
		}
		return true
	}
}

// CreateOrFilter creates a filter that combines multiple filters with OR logic
func CreateOrFilter(filters ...FilterFunc) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		for _, filter := range filters {
			if filter(level, message, fields) {
				return true
			}
		}
		return false
	}
}

// CreateNotFilter creates a filter that inverts another filter's decision
func CreateNotFilter(filter FilterFunc) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		return !filter(level, message, fields)
	}
}

// CreateFieldRangeFilter creates a filter for numeric field values within a range
func CreateFieldRangeFilter(field string, min, max float64) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}

		val, exists := fields[field]
		if !exists {
			return false
		}

		// Try to convert to float64
		var numVal float64
		switch v := val.(type) {
		case float64:
			numVal = v
		case float32:
			numVal = float64(v)
		case int:
			numVal = float64(v)
		case int64:
			numVal = float64(v)
		case int32:
			numVal = float64(v)
		default:
			return false
		}

		return numVal >= min && numVal <= max
	}
}

// CreateTimeWindowFilter creates a filter that only logs during specific time windows
func CreateTimeWindowFilter(startHour, endHour int) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		hour := time.Now().Hour()
		if startHour <= endHour {
			return hour >= startHour && hour < endHour
		}
		// Handle wrap around midnight
		return hour >= startHour || hour < endHour
	}
}

// CreateRateLimitFilter creates a filter that limits log frequency
func CreateRateLimitFilter(maxPerSecond float64) FilterFunc {
	// Simple token bucket implementation
	tokens := maxPerSecond
	lastRefill := time.Now()
	var mu sync.Mutex

	return func(level int, message string, fields map[string]interface{}) bool {
		mu.Lock()
		defer mu.Unlock()

		// Refill tokens
		now := time.Now()
		elapsed := now.Sub(lastRefill).Seconds()
		tokensToAdd := elapsed * maxPerSecond

		tokens = tokens + tokensToAdd
		if tokens > maxPerSecond {
			tokens = maxPerSecond
		}
		lastRefill = now

		// Check if we have tokens
		if tokens >= 1.0 {
			tokens--
			return true
		}

		return false
	}
}

// CreateFieldPrefixFilter creates a filter for fields with specific prefixes
func CreateFieldPrefixFilter(prefix string) FilterFunc {
	return func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}

		for key := range fields {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	}
}
