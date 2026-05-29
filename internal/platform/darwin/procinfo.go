package darwin

/*
#include <libproc.h>
#include <sys/sysctl.h>
#include <string.h>
#include <stdlib.h>

// getCWD returns the current working directory of a process via proc_pidinfo.
// The caller must free the returned string.
static char* getCWD(int pid) {
    struct proc_vnodepathinfo vpi;
    int ret = proc_pidinfo(pid, PROC_PIDVNODEPATHINFO, 0, &vpi, sizeof(vpi));
    if (ret <= 0) {
        return strdup("");
    }
    return strdup(vpi.pvi_cdir.vip_path);
}

// getChildren returns the PIDs of processes whose parent is ppid.
// outCount is set to the number of PIDs returned.
// The caller must free the returned array.
static int* getChildren(int ppid, int *outCount) {
    int mib[4] = {CTL_KERN, KERN_PROC, KERN_PROC_ALL, 0};
    size_t size = 0;
    *outCount = 0;

    if (sysctl(mib, 3, NULL, &size, NULL, 0) < 0) return NULL;
    struct kinfo_proc *procs = (struct kinfo_proc *)malloc(size);
    if (!procs) return NULL;
    if (sysctl(mib, 3, procs, &size, NULL, 0) < 0) {
        free(procs);
        return NULL;
    }

    int nprocs = (int)(size / sizeof(struct kinfo_proc));
    int *children = (int *)malloc(sizeof(int) * nprocs);
    if (!children) {
        free(procs);
        return NULL;
    }

    int count = 0;
    for (int i = 0; i < nprocs; i++) {
        if (procs[i].kp_eproc.e_ppid == ppid) {
            children[count++] = procs[i].kp_proc.p_pid;
        }
    }

    free(procs);
    *outCount = count;
    return children;
}
*/
import "C"
import "unsafe"

// GetProcessCWD returns the current working directory of the deepest child
// process under the given PID. This walks down the process tree so that for
// a terminal (PID) -> shell -> claude-code chain, we get the CWD of the
// innermost process that's actually doing work.
func GetProcessCWD(pid int) string {
	if pid <= 0 {
		return ""
	}

	// Walk down to the deepest child (max 10 levels to avoid loops).
	current := pid
	for i := 0; i < 10; i++ {
		var count C.int
		children := C.getChildren(C.int(current), &count)
		if children == nil || count == 0 {
			if children != nil {
				C.free(unsafe.Pointer(children))
			}
			break
		}
		// Pick the first child. For terminal -> shell this is usually correct.
		current = int(*children)
		C.free(unsafe.Pointer(children))
	}

	cwd := C.getCWD(C.int(current))
	defer C.free(unsafe.Pointer(cwd))
	return C.GoString(cwd)
}
