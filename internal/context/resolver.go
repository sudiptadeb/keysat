package context

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"



	"github.com/sudiptadeb/keysat/internal/platform/darwin"
)

// TypingContext describes the current typing context.
type TypingContext struct {
	AppName   string
	BundleID  string
	AppType   AppType
	Domain    string
	Directory string
}

// Resolver polls the frontmost app and merges in domain/directory from reporters.
type Resolver struct {
	mu            sync.RWMutex
	currentApp    darwin.FrontApp
	lastDomain    string
	lastDirectory string
	pollInterval  time.Duration
	stopCh        chan struct{}
	stopOnce      sync.Once

	OnContextChange func(old, new TypingContext)
}

// NewResolver creates a Resolver with a default 500ms poll interval.
func NewResolver() *Resolver {
	return &Resolver{
		pollInterval: 500 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}
}

// Start begins polling the frontmost app and listening on the Unix socket.
func (r *Resolver) Start() {
	go r.poll()
	go r.listenDirSocket()
}

// Stop terminates the polling goroutine and socket listener.
func (r *Resolver) Stop() {
	r.stopOnce.Do(func() { close(r.stopCh) })
}

// Current returns the current typing context.
// If the app is a browser the Domain field is populated.
// If the app is a terminal the Directory field is populated.
func (r *Resolver) Current() TypingContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.buildContext()
}

// SetDomain updates the last known browser domain (called by web reporter).
func (r *Resolver) SetDomain(domain string) {
	r.mu.Lock()
	r.lastDomain = domain
	r.mu.Unlock()
}

// SetDirectory updates the last known shell directory (called by shell hook reporter).
func (r *Resolver) SetDirectory(path string) {
	r.mu.Lock()
	r.lastDirectory = path
	r.mu.Unlock()
}

// poll runs in a goroutine, checking for frontmost app changes.
func (r *Resolver) poll() {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.update()
		}
	}
}

// update checks if the frontmost app has changed and fires the callback.
// It also polls the CWD of terminal apps so directory tracking works even
// when the shell hook (precmd) doesn't fire (e.g. inside Claude Code).
func (r *Resolver) update() {
	front := darwin.GetFrontmostApp()

	// If the frontmost app is a terminal, poll its child process tree for
	// the current working directory. This keeps directory context fresh
	// even when long-running programs suppress the shell prompt hook.
	if ClassifyApp(front.BundleID) == AppTypeTerminal && front.PID > 0 {
		if dir := darwin.GetProcessCWD(front.PID); dir != "" {
			r.mu.Lock()
			r.lastDirectory = dir
			r.mu.Unlock()
		}
	}

	r.mu.Lock()
	prev := r.currentApp
	r.currentApp = front

	if prev.BundleID == front.BundleID {
		r.mu.Unlock()
		return
	}

	oldCtx := r.buildContextForApp(prev)
	newCtx := r.buildContextForApp(front)
	cb := r.OnContextChange
	r.mu.Unlock()

	if cb != nil {
		cb(oldCtx, newCtx)
	}
}

// sockPath returns ~/.keysat/hook.sock.
func sockPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".keysat", "hook.sock")
}

// listenDirSocket creates a Unix socket at ~/.keysat/hook.sock.
// The shell hook writes the current directory to it — instant, no polling.
func (r *Resolver) listenDirSocket() {
	path := sockPath()
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		slog.Error("failed to create hook socket", "err", err)
		return
	}
	slog.Info("hook socket listening", "path", path)

	go func() {
		<-r.stopCh
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 4096)
			n, _ := c.Read(buf)
			dir := strings.TrimSpace(string(buf[:n]))
			if dir != "" {
				r.mu.Lock()
				r.lastDirectory = dir
				r.mu.Unlock()
			}
		}(conn)
	}
}

// buildContext creates a TypingContext from the current state.
// Caller must hold at least a read lock.
func (r *Resolver) buildContext() TypingContext {
	return r.buildContextForApp(r.currentApp)
}

// buildContextForApp creates a TypingContext for a given FrontApp.
// Caller must hold at least a read lock.
func (r *Resolver) buildContextForApp(app darwin.FrontApp) TypingContext {
	appType := ClassifyApp(app.BundleID)
	ctx := TypingContext{
		AppName:  app.Name,
		BundleID: app.BundleID,
		AppType:  appType,
	}
	if appType == AppTypeBrowser {
		ctx.Domain = r.lastDomain
	}
	if appType == AppTypeTerminal {
		ctx.Directory = r.lastDirectory
	}
	return ctx
}
