#ifndef APPINFO_H
#define APPINFO_H

typedef struct {
    const char *name;
    const char *bundleID;
    int pid;
} AppInfo;

AppInfo getFrontmostApp(void);
void freeAppInfo(AppInfo info);

#endif
