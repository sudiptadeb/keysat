package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "appinfo.h"
*/
import "C"

// FrontApp holds information about the frontmost application.
type FrontApp struct {
	Name     string
	BundleID string
	PID      int
}

// GetFrontmostApp returns info about the currently focused application.
func GetFrontmostApp() FrontApp {
	info := C.getFrontmostApp()
	defer C.freeAppInfo(info)
	return FrontApp{
		Name:     C.GoString(info.name),
		BundleID: C.GoString(info.bundleID),
		PID:      int(info.pid),
	}
}
