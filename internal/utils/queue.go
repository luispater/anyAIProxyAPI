package utils

import (
	"errors"
	"sync"
)

// Queue represents a thread-safe queue
type Queue[T any] struct {
	items []T
	mutex sync.Mutex
	cond  *sync.Cond
}

// NewQueue creates a new thread-safe queue
func NewQueue[T any]() *Queue[T] {
	q := &Queue[T]{items: make([]T, 0)}
	q.cond = sync.NewCond(&q.mutex)
	return q
}

// Enqueue adds an item to the end of the queue
func (q *Queue[T]) Enqueue(item T) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items = append(q.items, item)
	q.cond.Signal() // Wake up one waiting goroutine
}

// Dequeue removes and returns the item at the front of the queue
// Returns an error if the queue is empty (non-blocking)
func (q *Queue[T]) Dequeue() (T, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	var zero T
	if len(q.items) == 0 {
		return zero, errors.New("queue is empty")
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, nil
}

// DequeueBlocking removes and returns the item at the front of the queue
// Blocks until an item is available
func (q *Queue[T]) DequeueBlocking() T {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Wait until there's at least one item in the queue
	for len(q.items) == 0 {
		q.cond.Wait()
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item
}

// Peek returns the item at the front without removing it
func (q *Queue[T]) Peek() (T, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	var zero T
	if len(q.items) == 0 {
		return zero, errors.New("queue is empty")
	}
	return q.items[0], nil
}

// IsEmpty checks if the queue is empty
func (q *Queue[T]) IsEmpty() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.items) == 0
}

// Clear the queue
func (q *Queue[T]) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items = make([]T, 0)
}

// Size returns the number of items in the queue
func (q *Queue[T]) Size() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.items)
}
