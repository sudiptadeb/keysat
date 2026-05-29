package capture

// KeyEvent represents a single keystroke captured from the OS.
type KeyEvent struct {
	Char      rune
	Keycode   uint16
	Timestamp int64 // unix milliseconds
	IsRepeat  bool
}

// Capturer is the platform-specific keystroke capture interface.
type Capturer interface {
	Start(events chan<- KeyEvent) error
	Stop()
}
