#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>
#include "appinfo.h"

AppInfo getFrontmostApp(void) {
    AppInfo info;
    info.name = strdup("");
    info.bundleID = strdup("");
    info.pid = 0;

    @autoreleasepool {
        NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (app) {
            NSString *name = [app localizedName];
            NSString *bundleID = [app bundleIdentifier];
            if (name) {
                free((void *)info.name);
                info.name = strdup([name UTF8String]);
            }
            if (bundleID) {
                free((void *)info.bundleID);
                info.bundleID = strdup([bundleID UTF8String]);
            }
            info.pid = [app processIdentifier];
        }
    }
    return info;
}

void freeAppInfo(AppInfo info) {
    free((void *)info.name);
    free((void *)info.bundleID);
}
