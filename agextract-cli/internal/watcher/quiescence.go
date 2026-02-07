package watcher

import (
	"sync"
	"time"
)

const DefaultQuiescenceDuration = 60 * time.Second

// QuiescenceTracker manages per-file debounce timers.
// When a file stops being written to for the quiescence duration,
// the callback fires.
type QuiescenceTracker struct {
	mu       sync.Mutex
	timers   map[string]*time.Timer
	duration time.Duration
	callback func(filePath, tool string)
	toolMap  map[string]string // filePath -> tool
}

func NewQuiescenceTracker(duration time.Duration, callback func(filePath, tool string)) *QuiescenceTracker {
	return &QuiescenceTracker{
		timers:   make(map[string]*time.Timer),
		duration: duration,
		callback: callback,
		toolMap:  make(map[string]string),
	}
}

// Touch resets the quiescence timer for a file.
func (q *QuiescenceTracker) Touch(filePath, tool string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.toolMap[filePath] = tool

	if timer, ok := q.timers[filePath]; ok {
		timer.Stop()
	}

	q.timers[filePath] = time.AfterFunc(q.duration, func() {
		q.mu.Lock()
		delete(q.timers, filePath)
		t := q.toolMap[filePath]
		q.mu.Unlock()

		q.callback(filePath, t)
	})
}

// Stop cancels all pending timers.
func (q *QuiescenceTracker) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, timer := range q.timers {
		timer.Stop()
	}
	q.timers = make(map[string]*time.Timer)
}
