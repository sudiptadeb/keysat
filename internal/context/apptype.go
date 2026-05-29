package context

import "strings"

// AppType represents the type of application.
type AppType string

const (
	AppTypeBrowser  AppType = "browser"
	AppTypeTerminal AppType = "terminal"
	AppTypeEditor   AppType = "editor"
	AppTypeOther    AppType = "other"
)

var knownBrowsers = map[string]bool{
	"com.google.Chrome":          true,
	"com.apple.Safari":           true,
	"org.mozilla.firefox":        true,
	"com.brave.Browser":          true,
	"com.microsoft.edgemac":      true,
	"company.thebrowser.Browser": true,
	"com.operasoftware.Opera":    true,
	"com.primeum.Browser":         true,
}

var knownTerminals = map[string]bool{
	"com.apple.Terminal":       true,
	"com.googlecode.iterm2":    true,
	"io.alacritty":            true,
	"com.github.wez.wezterm":  true,
	"dev.warp.Warp-Stable":    true,
	"net.kovidgoyal.kitty":    true,
	"com.mitchellh.ghostty":   true,
}

var knownEditors = map[string]bool{
	"com.microsoft.VSCode": true,
	"com.sublimetext.4":    true,
	"com.apple.dt.Xcode":   true,
}

// ClassifyApp returns the app type based on bundle ID.
func ClassifyApp(bundleID string) AppType {
	if knownBrowsers[bundleID] {
		return AppTypeBrowser
	}
	if knownTerminals[bundleID] {
		return AppTypeTerminal
	}
	if knownEditors[bundleID] {
		return AppTypeEditor
	}
	// JetBrains IDEs all share the com.jetbrains.intellij prefix.
	if strings.HasPrefix(bundleID, "com.jetbrains.intellij") {
		return AppTypeEditor
	}
	return AppTypeOther
}
