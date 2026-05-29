.PHONY: build run clean start cert web

# Optional, untracked local overrides (e.g. APP_BUNDLE for your environment).
-include Makefile.local

BINARY=keysatd
SIGN_ID ?= keysat-dev
WEB_ADDR ?= 127.0.0.1:7890
# Where the signed .app bundle is built. Some setups require a specific
# location; override APP_BUNDLE in an (untracked) Makefile.local.
APP_BUNDLE ?= build/KeySat.app
APP_BINARY=$(APP_BUNDLE)/Contents/MacOS/$(BINARY)

build:
	go build -tags fts5 -o $(BINARY) ./cmd/keysatd

run: build
	./$(BINARY)

# One-time: create the stable self-signed code-signing identity so macOS keeps
# the Accessibility / Input Monitoring grant across rebuilds (ad-hoc signing
# changes identity every build and drops the grant).
cert:
	./scripts/create-signing-cert.sh $(SIGN_ID)

# Run the read-only dashboard viewer — serves the database with NO capture
# pipeline, so it needs no Accessibility permission. Open http://$(WEB_ADDR)
web:
	go build -tags fts5 -o $(BINARY)-webtest ./cmd/keysat-webtest
	./$(BINARY)-webtest -addr $(WEB_ADDR)

# Build the signed .app bundle and run it.
start:
	go build -tags fts5 -o $(APP_BINARY) ./cmd/keysatd
	codesign -f -s "$(SIGN_ID)" $(APP_BUNDLE)
	$(APP_BINARY)

clean:
	rm -f $(BINARY)
	rm -rf $(APP_BUNDLE)

PLIST_TEMPLATE=com.keysat.daemon.plist.template
PLIST_DST=$(HOME)/Library/LaunchAgents/com.keysat.daemon.plist

install: start-service install-hook

# Build and run in the background from your shell (no launchd agent).
bg: stop-service
	go build -tags fts5 -o $(APP_BINARY) ./cmd/keysatd
	codesign -f -s "$(SIGN_ID)" $(APP_BUNDLE)
	mkdir -p $(HOME)/.keysat/logs
	nohup $(APP_BINARY) > $(HOME)/.keysat/logs/stdout.log 2>&1 &
	@echo "$$!" > $(HOME)/.keysat/keysatd.pid
	@echo "keysat running in background (pid $$(cat $(HOME)/.keysat/keysatd.pid))"
	@echo "  logs: make logs"

start-service: stop-service
	go build -tags fts5 -o $(APP_BINARY) ./cmd/keysatd
	codesign -f -s "$(SIGN_ID)" $(APP_BUNDLE)
	mkdir -p $(HOME)/.keysat/logs
	sed -e 's#__APP_BINARY__#$(abspath $(APP_BINARY))#' -e 's#__LOG_DIR__#$(HOME)/.keysat/logs#' $(PLIST_TEMPLATE) > $(PLIST_DST)
	launchctl load $(PLIST_DST)
	@echo "keysat daemon installed and running"
	@echo "  logs: make logs"

stop-service:
	-launchctl unload $(PLIST_DST) 2>/dev/null
	-kill $$(cat $(HOME)/.keysat/keysatd.pid 2>/dev/null) 2>/dev/null
	-lsof -i :7890 -t 2>/dev/null | xargs kill 2>/dev/null
	-rm -f $(HOME)/.keysat/keysatd.pid
	@echo "keysat daemon stopped"

uninstall: stop-service
	rm -f $(PLIST_DST)
	rm -rf $(APP_BUNDLE)
	@echo "keysat uninstalled"

logs:
	tail -f $(HOME)/.keysat/logs/stdout.log

install-hook:
	@echo "Add this to your .bashrc or .zshrc:"
	@echo "  source $(PWD)/shell/keysat-hook.sh"

.PHONY: install-extension
install-extension:
	@echo "Load the extension from: $(PWD)/extension/"
	@echo "1. Open chrome://extensions"
	@echo "2. Enable Developer Mode"
	@echo "3. Click 'Load unpacked'"
	@echo "4. Select the extension/ directory"
