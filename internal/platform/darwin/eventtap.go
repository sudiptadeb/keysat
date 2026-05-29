package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon
#include "eventtap.h"
*/
import "C"

import (
	"fmt"
	"time"

	"github.com/sudiptadeb/keysat/internal/capture"
)

// eventChan is the package-level channel that the C callback writes into.
var eventChan chan<- capture.KeyEvent

//export goKeyEventCallback
func goKeyEventCallback(ch C.UniChar, keycode C.CGKeyCode, isRepeat C.int) {
	if eventChan == nil {
		return
	}
	eventChan <- capture.KeyEvent{
		Char:      rune(ch),
		Keycode:   uint16(keycode),
		Timestamp: time.Now().UnixMilli(),
		IsRepeat:  isRepeat != 0,
	}
}

// DarwinCapturer implements capture.Capturer using a macOS CGEventTap.
type DarwinCapturer struct{}

func (d *DarwinCapturer) Start(events chan<- capture.KeyEvent) error {
	if IsSecureInputEnabled() {
		fmt.Println("warning: secure input is currently enabled (e.g. password field focused), capture will resume when it's released")
	}
	eventChan = events
	go C.startEventTap()
	return nil
}

func (d *DarwinCapturer) Stop() {
	C.stopEventTap()
	eventChan = nil
}

// IsSecureInputEnabled reports whether secure event input is active.
func IsSecureInputEnabled() bool {
	return C.isSecureInputEnabled() != 0
}
