package flexlog

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// DispatcherMode defines how messages are distributed to workers
type DispatcherMode int

const (
	// DispatcherModeSingle uses a single worker (default, current behavior)
	DispatcherModeSingle DispatcherMode = iota
	// DispatcherModeRoundRobin distributes messages across workers in round-robin
	DispatcherModeRoundRobin
	// DispatcherModeLoadBalanced distributes based on worker load
	DispatcherModeLoadBalanced
)

// WorkerPool manages multiple dispatcher workers for improved throughput
type WorkerPool struct {
	workers       []*Worker
	mode          DispatcherMode
	numWorkers    int
	nextWorker    atomic.Uint64
	logger        *FlexLog
	wg            sync.WaitGroup
	messageQueues []chan LogMessage
}

// Worker represents a single message processing worker
type Worker struct {
	id       int
	queue    chan LogMessage
	logger   *FlexLog
	stats    WorkerStats
	shutdown chan struct{}
}

// WorkerStats tracks performance metrics for a worker
type WorkerStats struct {
	messagesProcessed atomic.Uint64
	bytesWritten      atomic.Uint64
	errors            atomic.Uint64
	queueDepth        atomic.Int32
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(logger *FlexLog, numWorkers int, mode DispatcherMode) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	wp := &WorkerPool{
		workers:       make([]*Worker, numWorkers),
		mode:          mode,
		numWorkers:    numWorkers,
		logger:        logger,
		messageQueues: make([]chan LogMessage, numWorkers),
	}

	// Create workers with dedicated queues
	queueSize := logger.channelSize / numWorkers
	if queueSize < 100 {
		queueSize = 100
	}

	for i := 0; i < numWorkers; i++ {
		queue := make(chan LogMessage, queueSize)
		wp.messageQueues[i] = queue
		
		worker := &Worker{
			id:       i,
			queue:    queue,
			logger:   logger,
			shutdown: make(chan struct{}),
		}
		wp.workers[i] = worker
	}

	return wp
}

// Start begins all workers in the pool
func (wp *WorkerPool) Start() {
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go wp.runWorker(worker)
	}
}

// Stop gracefully shuts down all workers
func (wp *WorkerPool) Stop() {
	// Signal all workers to stop
	for _, worker := range wp.workers {
		close(worker.shutdown)
	}
	
	// Close all message queues
	for _, queue := range wp.messageQueues {
		close(queue)
	}
	
	// Wait for all workers to finish
	wp.wg.Wait()
}

// Dispatch sends a message to the appropriate worker based on the dispatch mode
func (wp *WorkerPool) Dispatch(msg LogMessage) bool {
	switch wp.mode {
	case DispatcherModeRoundRobin:
		return wp.dispatchRoundRobin(msg)
	case DispatcherModeLoadBalanced:
		return wp.dispatchLoadBalanced(msg)
	default:
		return wp.dispatchSingle(msg)
	}
}

// dispatchSingle sends all messages to the first worker (legacy behavior)
func (wp *WorkerPool) dispatchSingle(msg LogMessage) bool {
	select {
	case wp.messageQueues[0] <- msg:
		wp.workers[0].stats.queueDepth.Add(1)
		return true
	default:
		return false
	}
}

// dispatchRoundRobin distributes messages across workers in round-robin fashion
func (wp *WorkerPool) dispatchRoundRobin(msg LogMessage) bool {
	// Get next worker index atomically
	idx := wp.nextWorker.Add(1) % uint64(wp.numWorkers)
	
	select {
	case wp.messageQueues[idx] <- msg:
		wp.workers[idx].stats.queueDepth.Add(1)
		return true
	default:
		// Try next worker if current is full
		for i := 0; i < wp.numWorkers; i++ {
			nextIdx := (idx + uint64(i)) % uint64(wp.numWorkers)
			select {
			case wp.messageQueues[nextIdx] <- msg:
				wp.workers[nextIdx].stats.queueDepth.Add(1)
				return true
			default:
				continue
			}
		}
		return false
	}
}

// dispatchLoadBalanced sends messages to the least loaded worker
func (wp *WorkerPool) dispatchLoadBalanced(msg LogMessage) bool {
	// Find worker with smallest queue
	minDepth := int32(^uint32(0) >> 1) // Max int32
	minIdx := 0
	
	for i, worker := range wp.workers {
		depth := worker.stats.queueDepth.Load()
		if depth < minDepth {
			minDepth = depth
			minIdx = i
		}
	}
	
	// Try to send to least loaded worker
	select {
	case wp.messageQueues[minIdx] <- msg:
		wp.workers[minIdx].stats.queueDepth.Add(1)
		return true
	default:
		// Fall back to round robin if preferred worker is full
		return wp.dispatchRoundRobin(msg)
	}
}

// runWorker processes messages for a single worker
func (wp *WorkerPool) runWorker(worker *Worker) {
	defer wp.wg.Done()
	
	// Get destinations once to avoid repeated locking
	wp.logger.mu.RLock()
	destinations := make([]*Destination, len(wp.logger.Destinations))
	copy(destinations, wp.logger.Destinations)
	wp.logger.mu.RUnlock()
	
	for {
		select {
		case msg, ok := <-worker.queue:
			if !ok {
				return // Queue closed
			}
			
			// Update queue depth
			worker.stats.queueDepth.Add(-1)
			
			// Process message to all destinations
			for _, dest := range destinations {
				// Skip disabled destinations
				dest.mu.RLock()
				enabled := dest.Enabled
				dest.mu.RUnlock()
				
				if !enabled {
					continue
				}
				
				// Process the message
				wp.logger.processMessage(msg, dest)
			}
			
			// Update stats
			worker.stats.messagesProcessed.Add(1)
			
		case <-worker.shutdown:
			// Drain remaining messages before shutting down
			for {
				select {
				case msg, ok := <-worker.queue:
					if !ok {
						return
					}
					worker.stats.queueDepth.Add(-1)
					
					// Process remaining messages
					for _, dest := range destinations {
						dest.mu.RLock()
						enabled := dest.Enabled
						dest.mu.RUnlock()
						
						if enabled {
							wp.logger.processMessage(msg, dest)
						}
					}
					worker.stats.messagesProcessed.Add(1)
				default:
					return
				}
			}
		}
	}
}

// GetStats returns statistics for all workers
func (wp *WorkerPool) GetStats() []WorkerStats {
	stats := make([]WorkerStats, len(wp.workers))
	for i, worker := range wp.workers {
		stats[i] = WorkerStats{
			messagesProcessed: atomic.Uint64{},
			bytesWritten:      atomic.Uint64{},
			errors:            atomic.Uint64{},
			queueDepth:        atomic.Int32{},
		}
		// Copy values
		stats[i].messagesProcessed.Store(worker.stats.messagesProcessed.Load())
		stats[i].bytesWritten.Store(worker.stats.bytesWritten.Load())
		stats[i].errors.Store(worker.stats.errors.Load())
		stats[i].queueDepth.Store(worker.stats.queueDepth.Load())
	}
	return stats
}

// EnableMultipleDispatchers configures the logger to use multiple dispatcher workers
func (f *FlexLog) EnableMultipleDispatchers(numWorkers int, mode DispatcherMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if f.closed {
		return ErrLoggerClosed
	}
	
	// Note: This would require significant refactoring of the existing
	// single dispatcher model. For now, this serves as a design template
	// for future implementation.
	
	// The implementation would:
	// 1. Replace the single messageDispatcher goroutine
	// 2. Create a WorkerPool
	// 3. Modify log methods to use wp.Dispatch instead of direct channel send
	
	return nil
}