package api

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

// RequestQueue manages the sequential processing of chat completion requests
type RequestQueue struct {
	tasks     chan *RequestTask
	mu        sync.RWMutex
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	processor TaskProcessor
}

// TaskProcessor defines the interface for processing tasks
type TaskProcessor interface {
	ProcessTask(ctx context.Context, task *RequestTask) *TaskResponse
}

// NewRequestQueue creates a new request queue
func NewRequestQueue(processor TaskProcessor) *RequestQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &RequestQueue{
		tasks:     make(chan *RequestTask, 100), // Buffer for 100 requests
		ctx:       ctx,
		cancel:    cancel,
		processor: processor,
	}
}

// Start begins processing requests from the queue
func (q *RequestQueue) Start() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return fmt.Errorf("queue is already running")
	}

	q.running = true
	q.wg.Add(1)

	go q.processLoop()
	log.Debug("Request queue started")
	return nil
}

// Stop stops the request queue and waits for current task to complete
func (q *RequestQueue) Stop() error {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return fmt.Errorf("queue is not running")
	}
	q.running = false
	q.mu.Unlock()

	q.cancel()
	close(q.tasks)
	q.wg.Wait()
	log.Debug("Request queue stopped")
	return nil
}

// AddTask adds a new task to the queue
func (q *RequestQueue) AddTask(task *RequestTask) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if !q.running {
		return fmt.Errorf("queue is not running")
	}

	select {
	case q.tasks <- task:
		log.Debugf("Task %s added to queue", task.ID)
		return nil
	case <-q.ctx.Done():
		return fmt.Errorf("queue is shutting down")
	default:
		return fmt.Errorf("queue is full")
	}
}

// processLoop is the main processing loop that handles tasks sequentially
func (q *RequestQueue) processLoop() {
	defer q.wg.Done()

	for {
		select {
		case task, ok := <-q.tasks:
			if !ok {
				log.Println("Task channel closed, stopping process loop")
				return
			}

			log.Debugf("Processing task %s", task.ID)
			startTime := time.Now()

			// Process the task
			response := q.processor.ProcessTask(q.ctx, task)

			duration := time.Since(startTime)
			log.Debugf("Task %s completed in %v", task.ID, duration)

			// Send response back to the handler
			select {
			case task.Response <- response:
			case <-q.ctx.Done():
				log.Debugf("Context cancelled while sending response for task %s", task.ID)
				return
			case <-time.After(30 * time.Second):
				log.Debugf("Timeout sending response for task %s", task.ID)
			}

		case <-q.ctx.Done():
			log.Debug("Context cancelled, stopping process loop")
			return
		}
	}
}

// GetQueueLength returns the current number of tasks in the queue
func (q *RequestQueue) GetQueueLength() int {
	return len(q.tasks)
}

// IsRunning returns whether the queue is currently running
func (q *RequestQueue) IsRunning() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.running
}
