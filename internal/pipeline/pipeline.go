package pipeline

import (
	"log/slog"
	"sync"
	"time"

	"github.com/sudiptadeb/keysat/internal/capture"
	kcontext "github.com/sudiptadeb/keysat/internal/context"
	"github.com/sudiptadeb/keysat/internal/platform/darwin"
	"github.com/sudiptadeb/keysat/internal/storage"
)

// Pipeline is the main orchestrator: capture -> words -> context -> store.
type Pipeline struct {
	capturer  capture.Capturer
	assembler *capture.WordAssembler
	resolver  *kcontext.Resolver
	db        *storage.DB
	buffer    *Buffer
	events    chan capture.KeyEvent
	stopCh    chan struct{}
	stopOnce  sync.Once
	logger    *slog.Logger

	// Session state, protected by mu.
	mu               sync.Mutex
	currentSessionID int64
	lastContext      kcontext.TypingContext
	sessionWords     int
	sessionKeys      int
}

// NewPipeline creates a Pipeline wired to the given DB and context Resolver.
func NewPipeline(db *storage.DB, resolver *kcontext.Resolver) *Pipeline {
	p := &Pipeline{
		capturer:  &darwin.DarwinCapturer{},
		assembler: capture.NewWordAssembler(),
		resolver:  resolver,
		db:        db,
		events:    make(chan capture.KeyEvent, 256),
		stopCh:    make(chan struct{}),
		logger:    slog.Default(),
	}

	p.buffer = NewBuffer(100, 2*time.Second, p.flushToDB)

	return p
}

// Start checks platform permissions, starts the context resolver and
// keystroke capturer, then launches the main event-processing loop.
func (p *Pipeline) Start() error {
	if err := darwin.CheckPermissions(); err != nil {
		return err
	}

	p.resolver.Start()

	if err := p.capturer.Start(p.events); err != nil {
		p.resolver.Stop()
		return err
	}

	go p.run()
	return nil
}

// Stop signals the event loop to exit, flushes remaining data,
// ends the active session, and tears down the capturer and resolver.
func (p *Pipeline) Stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })

	p.buffer.Stop()

	p.mu.Lock()
	sessionID := p.currentSessionID
	p.mu.Unlock()

	if sessionID != 0 {
		p.endSession()
	}

	p.capturer.Stop()
	p.resolver.Stop()
}

// run is the main event loop, launched as a goroutine by Start.
func (p *Pipeline) run() {
	assemblerTick := time.NewTicker(p.assembler.FlushTimeout())
	defer assemblerTick.Stop()

	for {
		select {
		case <-p.stopCh:
			return

		case evt := <-p.events:
			p.handleEvent(evt)

		case <-assemblerTick.C:
			// Flush assembler if idle (e.g. user typed partial word then stopped).
			if time.Since(p.assembler.LastEvent()) >= p.assembler.FlushTimeout() {
				if word := p.assembler.Flush(); word != "" {
					isHashed := false
					if capture.IsLikelyPassword(word) {
						word = capture.HashWord(word)
						isHashed = true
					}
					p.buffer.Add(WordEntry{
						Word:     word,
						IsHashed: isHashed,
						TypedAt:  time.Now().UnixMilli(),
					})
				}
			}
		}
	}
}

// contextEqual returns true if two typing contexts represent the same
// application environment.
func contextEqual(a, b kcontext.TypingContext) bool {
	return a.BundleID == b.BundleID &&
		a.Domain == b.Domain &&
		a.Directory == b.Directory
}

// handleEvent processes a single KeyEvent through the pipeline stages.
func (p *Pipeline) handleEvent(evt capture.KeyEvent) {
	// Skip the macOS login/lock screen — keystrokes there are password
	// entries and should never be recorded.
	ctx := p.resolver.Current()
	if ctx.BundleID == "com.apple.loginwindow" {
		return
	}

	p.buffer.AddKeystroke()

	// If secure input is active (e.g. password field), skip word assembly
	// entirely so we never accumulate sensitive characters.
	if darwin.IsSecureInputEnabled() {
		return
	}

	// Feed the event to the word assembler.
	words := p.assembler.Feed(evt)

	// Check for a context change before adding words.
	currentCtx := p.resolver.Current()
	p.mu.Lock()
	ctxChanged := !contextEqual(currentCtx, p.lastContext)
	p.mu.Unlock()

	if ctxChanged {
		p.onContextChange(currentCtx)
	}

	// Process each completed word.
	for _, w := range words {
		isHashed := false
		word := w

		if capture.IsLikelyPassword(w) {
			word = capture.HashWord(w)
			isHashed = true
		}

		p.buffer.Add(WordEntry{
			Word:     word,
			IsHashed: isHashed,
			TypedAt:  evt.Timestamp,
		})
	}
}

// onContextChange flushes the buffer for the old context, ends the old
// session, and starts a new session for the new context.
func (p *Pipeline) onContextChange(newCtx kcontext.TypingContext) {
	// Flush words belonging to the previous context.
	entries, keystrokes := p.buffer.Flush()
	if len(entries) > 0 || keystrokes > 0 {
		p.flushToDB(entries, keystrokes)
	}

	// End the previous session if one is active.
	p.mu.Lock()
	hadSession := p.currentSessionID != 0
	p.mu.Unlock()

	if hadSession {
		p.endSession()
	}

	// Record the new context and start a fresh session.
	p.mu.Lock()
	p.lastContext = newCtx
	p.mu.Unlock()

	p.startSession(newCtx)
}

// flushToDB is the buffer's flush callback. It writes accumulated word
// entries to the database under the current session.
func (p *Pipeline) flushToDB(entries []WordEntry, keystrokeCount int) {
	p.mu.Lock()
	sessionID := p.currentSessionID
	p.mu.Unlock()

	// If there is no session yet (first words before context poll),
	// start one from the current context.
	if sessionID == 0 {
		ctx := p.resolver.Current()
		p.mu.Lock()
		p.lastContext = ctx
		p.mu.Unlock()
		p.startSession(ctx)

		p.mu.Lock()
		sessionID = p.currentSessionID
		p.mu.Unlock()
	}

	if len(entries) > 0 {
		p.mu.Lock()
		ctx := p.lastContext
		p.mu.Unlock()

		appID, err := p.db.GetOrCreateApp(ctx.BundleID, ctx.AppName, string(ctx.AppType))
		if err != nil {
			p.logger.Error("get/create app", "err", err)
			return
		}
		domainID, err := p.db.GetOrCreateDomain(ctx.Domain)
		if err != nil {
			p.logger.Error("get/create domain", "err", err)
		}
		dirID, err := p.db.GetOrCreateDirectory(ctx.Directory)
		if err != nil {
			p.logger.Error("get/create directory", "err", err)
		}

		inserts := make([]storage.WordInsert, len(entries))
		for i, e := range entries {
			inserts[i] = storage.WordInsert{
				Word:        e.Word,
				IsHashed:    e.IsHashed,
				TypedAt:     e.TypedAt,
				SessionID:   sessionID,
				AppID:       appID,
				DomainID:    domainID,
				DirectoryID: dirID,
			}
		}

		if err := p.db.InsertWords(inserts); err != nil {
			p.logger.Error("failed to insert words", "err", err)
		}
	}

	// Update running session counts and persist to DB so stats queries
	// see the active session's data without waiting for EndSession.
	p.mu.Lock()
	p.sessionKeys += keystrokeCount
	p.sessionWords += len(entries)
	sid := p.currentSessionID
	keys := p.sessionKeys
	wds := p.sessionWords
	p.mu.Unlock()

	if sid != 0 {
		if err := p.db.UpdateSessionCounts(sid, keys, wds); err != nil {
			p.logger.Error("update session counts", "err", err)
		}
	}
}

// startSession creates a new typing session in the database for the given
// context, resolving (or creating) the app, domain, and directory IDs.
func (p *Pipeline) startSession(ctx kcontext.TypingContext) {
	appID, err := p.db.GetOrCreateApp(ctx.BundleID, ctx.AppName, string(ctx.AppType))
	if err != nil {
		p.logger.Error("failed to get/create app", "err", err)
		return
	}

	domainID, err := p.db.GetOrCreateDomain(ctx.Domain)
	if err != nil {
		p.logger.Error("failed to get/create domain", "err", err)
	}

	dirID, err := p.db.GetOrCreateDirectory(ctx.Directory)
	if err != nil {
		p.logger.Error("failed to get/create directory", "err", err)
	}

	sessionID, err := p.db.StartSession(appID, domainID, dirID, time.Now().UnixMilli())
	if err != nil {
		p.logger.Error("failed to start session", "err", err)
		return
	}

	p.mu.Lock()
	p.currentSessionID = sessionID
	p.sessionWords = 0
	p.sessionKeys = 0
	p.mu.Unlock()

	p.logger.Info("session started",
		"session_id", sessionID,
		"app", ctx.AppName,
		"bundle", ctx.BundleID,
	)
}

// endSession finalises the current typing session in the database.
func (p *Pipeline) endSession() {
	p.mu.Lock()
	sessionID := p.currentSessionID
	words := p.sessionWords
	keys := p.sessionKeys
	p.currentSessionID = 0
	p.sessionWords = 0
	p.sessionKeys = 0
	p.mu.Unlock()

	if sessionID == 0 {
		return
	}

	if err := p.db.EndSession(sessionID, time.Now().UnixMilli(), keys, words); err != nil {
		p.logger.Error("failed to end session", "err", err, "session_id", sessionID)
		return
	}

	p.logger.Info("session ended",
		"session_id", sessionID,
		"keystrokes", keys,
		"words", words,
	)
}
