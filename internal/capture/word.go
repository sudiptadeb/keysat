package capture

import (
	"sync"
	"time"
)

// WordAssembler converts a stream of KeyEvents into completed words.
type WordAssembler struct {
	buffer       []rune
	mu           sync.Mutex
	lastEvent    time.Time
	flushTimeout time.Duration
}

// NewWordAssembler creates a WordAssembler with a default 3s flush timeout.
func NewWordAssembler() *WordAssembler {
	return &WordAssembler{
		flushTimeout: 3 * time.Second,
	}
}

// delimiters is the set of characters that cause the buffer to flush.
const delimiters = " \t\n\r.,;:!?/\\()[]{}'\"`<>"

func isDelimiter(r rune) bool {
	for _, d := range delimiters {
		if r == d {
			return true
		}
	}
	return false
}

// Feed processes a KeyEvent and returns any completed words (usually 0 or 1).
func (w *WordAssembler) Feed(event KeyEvent) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastEvent = time.Now()

	// Backspace: keycode 51 on macOS, or char 8 (BS) / 127 (DEL).
	if event.Keycode == 51 || event.Char == 8 || event.Char == 127 {
		if len(w.buffer) > 0 {
			w.buffer = w.buffer[:len(w.buffer)-1]
		}
		return nil
	}

	// Escape: keycode 53 or char 27.
	if event.Keycode == 53 || event.Char == 27 {
		w.buffer = w.buffer[:0]
		return nil
	}

	// Delimiter: flush the current buffer as a word.
	if isDelimiter(event.Char) {
		word := w.flushLocked()
		if word != "" {
			return []string{word}
		}
		return nil
	}

	// Printable character: append to buffer.
	if event.Char >= 32 {
		w.buffer = append(w.buffer, event.Char)
	}

	return nil
}

// Flush forces the current buffer to be returned as a word.
// Returns empty string if the buffer is empty.
func (w *WordAssembler) Flush() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// flushLocked returns the buffer contents as a string and resets the buffer.
// Caller must hold w.mu.
func (w *WordAssembler) flushLocked() string {
	if len(w.buffer) == 0 {
		return ""
	}
	word := string(w.buffer)
	w.buffer = w.buffer[:0]
	return word
}

// LastEvent returns the time of the most recent Feed call.
func (w *WordAssembler) LastEvent() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastEvent
}

// FlushTimeout returns the configured inactivity timeout.
func (w *WordAssembler) FlushTimeout() time.Duration {
	return w.flushTimeout
}
