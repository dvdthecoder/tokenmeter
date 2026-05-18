package proxy

import (
	"net/http"
	"os"
	"strings"
)

// detectClient parses the request headers to identify which AI coding tool
// sent the request and which version it is.
func detectClient(req *http.Request) (name, version string) {
	ua := req.Header.Get("User-Agent")
	xClient := req.Header.Get("X-Client-Id")       // some tools set this
	xProduct := req.Header.Get("X-Product-Metadata") // Anthropic SDK extended header

	combined := strings.ToLower(ua + " " + xClient + " " + xProduct)

	switch {
	case strings.Contains(combined, "claude-cli"):
		name = "claude-code-cli"
		version = versionFromUA(ua, "claude-cli/")
	case strings.Contains(combined, "claude-code"):
		name = clientNameFromUA(combined, "claude-code")
		version = versionFromUA(ua, "claude-code/")
	case strings.Contains(combined, "cursor"):
		name, version = "cursor", versionFromUA(ua, "cursor/")
	case strings.Contains(combined, "windsurf"):
		name, version = "windsurf", versionFromUA(ua, "windsurf/")
	case strings.Contains(combined, "continue"):
		name, version = "vscode-continue", versionFromUA(ua, "continue/")
	case strings.Contains(combined, "cline"):
		name, version = "vscode-cline", versionFromUA(ua, "cline/")
	case strings.Contains(combined, "aider"):
		name, version = "aider", versionFromUA(ua, "aider/")
	case strings.Contains(combined, "codex"):
		name, version = "codex-cli", versionFromUA(ua, "codex/")
	case strings.Contains(combined, "opencode"):
		name, version = "opencode", versionFromUA(ua, "opencode/")
	case strings.Contains(combined, "anthropic-sdk-typescript"):
		// Raw TypeScript SDK — likely a custom agent or script.
		name, version = "anthropic-sdk-ts", versionFromUA(ua, "anthropic-sdk-typescript/")
	case strings.Contains(combined, "anthropic-sdk-python"):
		name, version = "anthropic-sdk-python", versionFromUA(ua, "anthropic-sdk-python/")
	case strings.Contains(combined, "python-httpx") || strings.Contains(combined, "python-requests"):
		name = "python-agent"
	default:
		if ua != "" {
			name = "unknown"
		}
	}
	return
}

// clientNameFromUA distinguishes CLI vs desktop app for Claude Code.
// Claude Code CLI sets a different product string than the Mac desktop app.
func clientNameFromUA(combined, base string) string {
	if strings.Contains(combined, "electron") || strings.Contains(combined, "darwin-arm64-app") {
		return base + "-app"
	}
	return base + "-cli"
}

// versionFromUA extracts a version string after a known prefix like "claude-code/".
func versionFromUA(ua, prefix string) string {
	lower := strings.ToLower(ua)
	idx := strings.Index(lower, strings.ToLower(prefix))
	if idx < 0 {
		return ""
	}
	rest := ua[idx+len(prefix):]
	end := strings.IndexAny(rest, " \t;,")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// systemUsername returns the OS user running the process, or the
// TOKENMETER_USER env var if set.
func systemUsername() string {
	if u := os.Getenv("TOKENMETER_USER"); u != "" {
		return u
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return os.Getenv("USERNAME") // Windows
}
