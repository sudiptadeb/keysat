package pipeline

import (
	"sync"
	"time"
)

// WordEntry represents a single word captured from the keyboard.
type WordEntry struct {
	Word     string
	IsHashed bool
	TypedAt  int64 // unix milliseconds
}

// Buffer accumulates word entries and flushes them periodically or when full.
type Buffer struct {
	mu             sync.Mutex
	entries        []WordEntry
	maxSize        int
	flushInterval  time.Duration
	flushFn        func([]WordEntry, int)
	timer          *time.Timer
	keystrokeCount int
}

// NewBuffer creates a Buffer that calls flushFn when the buffer is full or
// after flushInterval of inactivity. Pass 0 for maxSize to default to 100,
// and 0 for flushInterval to default to 2 seconds.
func NewBuffer(maxSize int, flushInterval time.Duration, flushFn func([]WordEntry, int)) *Buffer {
	if maxSize <= 0 {
		maxSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}

	b := &Buffer{
		entries:       make([]WordEntry, 0, maxSize),
		maxSize:       maxSize,
		flushInterval: flushInterval,
		flushFn:       flushFn,
	}

	b.timer = time.AfterFunc(flushInterval, b.timerFlush)
	b.timer.Stop() // start stopped; armed on first Add

	return b
}

// Add appends a word entry to the buffer. If the buffer reaches maxSize,
// it is flushed immediately. The inactivity timer is reset on each call.
func (b *Buffer) Add(entry WordEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = append(b.entries, entry)

	if len(b.entries) >= b.maxSize {
		b.flushLocked()
		return
	}

	// Reset the inactivity timer.
	b.timer.Reset(b.flushInterval)
}

// AddKeystroke increments the keystroke count for the current buffer period.
func (b *Buffer) AddKeystroke() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.keystrokeCount++
}

// Flush drains the buffer and returns the accumulated entries and keystroke
// count. The buffer and keystroke counter are reset.
func (b *Buffer) Flush() ([]WordEntry, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.drainLocked()
}

// Stop stops the inactivity timer and performs a final flush.
func (b *Buffer) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.timer.Stop()
	b.flushLocked()
}

// timerFlush is called by the inactivity timer in its own goroutine.
func (b *Buffer) timerFlush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked drains the buffer and calls flushFn outside the lock
// to avoid blocking Add/AddKeystroke during slow DB writes.
func (b *Buffer) flushLocked() {
	entries, keystrokes := b.drainLocked()
	if len(entries) == 0 && keystrokes == 0 {
		return
	}
	if b.flushFn != nil {
		b.mu.Unlock()
		b.flushFn(entries, keystrokes)
		b.mu.Lock()
	}
}

// drainLocked returns the buffered entries and keystroke count, then resets
// both. Caller must hold b.mu.
func (b *Buffer) drainLocked() ([]WordEntry, int) {
	entries := b.entries
	keystrokes := b.keystrokeCount

	b.entries = make([]WordEntry, 0, b.maxSize)
	b.keystrokeCount = 0
	b.timer.Stop()

	return entries, keystrokes
}
