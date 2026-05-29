#include "eventtap.h"
#include <Carbon/Carbon.h>

// Go callback declared in eventtap.go via //export.
extern void goKeyEventCallback(UniChar ch, CGKeyCode keycode, int isRepeat);

static CFMachPortRef eventTap = NULL;
static CFRunLoopSourceRef runLoopSource = NULL;
static CFRunLoopRef tapRunLoop = NULL;

static CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type,
                                CGEventRef event, void *userInfo) {
    (void)proxy;
    (void)userInfo;

    // If the tap is disabled by the system, re-enable it.
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        CGEventTapEnable(eventTap, true);
        return event;
    }

    if (type != kCGEventKeyDown) {
        return event;
    }

    CGKeyCode keycode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    int isRepeat = (int)CGEventGetIntegerValueField(event, kCGKeyboardEventAutorepeat);

    UniChar chars[4];
    UniCharCount len = 0;
    CGEventKeyboardGetUnicodeString(event, 4, &len, chars);

    UniChar ch = 0;
    if (len > 0) {
        ch = chars[0];
    }

    goKeyEventCallback(ch, keycode, isRepeat);

    return event;
}

void startEventTap(void) {
    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown);

    eventTap = CGEventTapCreate(
        kCGHIDEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly,
        mask,
        eventCallback,
        NULL
    );

    if (!eventTap) {
        return;
    }

    runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, eventTap, 0);
    tapRunLoop = CFRunLoopGetCurrent();

    CFRunLoopAddSource(tapRunLoop, runLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(eventTap, true);

    CFRunLoopRun();
}

void stopEventTap(void) {
    if (eventTap) {
        CGEventTapEnable(eventTap, false);
        if (runLoopSource) {
            CFRunLoopRemoveSource(tapRunLoop, runLoopSource, kCFRunLoopCommonModes);
            CFRelease(runLoopSource);
            runLoopSource = NULL;
        }
        CFRelease(eventTap);
        eventTap = NULL;
    }

    if (tapRunLoop) {
        CFRunLoopStop(tapRunLoop);
        tapRunLoop = NULL;
    }
}

int isSecureInputEnabled(void) {
    return (int)IsSecureEventInputEnabled();
}
