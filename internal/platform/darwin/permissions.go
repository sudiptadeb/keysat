package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>

static int checkAccessibilityTrusted(void) {
    return (int)AXIsProcessTrusted();
}

static int checkListenEventAccess(void) {
    return (int)CGPreflightListenEventAccess();
}
*/
import "C"

import "fmt"

// CheckPermissions verifies that the process has Accessibility and
// Input Monitoring permissions. Returns a descriptive error if either
// is missing, or nil if both are granted.
func CheckPermissions() error {
	trusted := C.checkAccessibilityTrusted() != 0
	listenOK := C.checkListenEventAccess() != 0

	if !trusted && !listenOK {
		return fmt.Errorf("missing permissions: enable Accessibility and Input Monitoring in System Settings > Privacy & Security")
	}
	if !trusted {
		return fmt.Errorf("missing permission: enable Accessibility in System Settings > Privacy & Security > Accessibility")
	}
	if !listenOK {
		return fmt.Errorf("missing permission: enable Input Monitoring in System Settings > Privacy & Security > Input Monitoring")
	}
	return nil
}
