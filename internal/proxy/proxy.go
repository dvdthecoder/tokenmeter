// Package proxy is the core HTTP interception engine.
// It wraps httputil.ReverseProxy, detects the LLM provider, forwards the
// request upstream, intercepts the response to extract token usage, then
// fans the UsageEvent out to all registered sinks.
package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/dvdthecoder/tokenmeter/internal/config"
	"github.com/dvdthecoder/tokenmeter/plugins/middleware"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

type contextKey int

const (
	ctxProvider   contextKey = iota
	ctxRequestID  contextKey = iota
	ctxStartTime  contextKey = iota
	ctxSessionID  contextKey = iota
)

// sessionTracker assigns synthetic session IDs to clients that don't send a
// session header (e.g. Claude Code CLI). Requests from the same key within
// sessionTimeout are grouped into one session; inactivity resets the bucket.
type sessionTracker struct {
	mu      sync.Mutex
	entries map[string]*sessionEntry
}

type sessionEntry struct {
	id       string
	lastSeen time.Time
}

const sessionTimeout = 30 * time.Minute

func (t *sessionTracker) get(key string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if e, ok := t.entries[key]; ok && now.Sub(e.lastSeen) < sessionTimeout {
		e.lastSeen = now
		return e.id
	}
	id := "syn-" + uuid.New().String()[:8]
	t.entries[key] = &sessionEntry{id: id, lastSeen: now}
	return id
}

// Proxy is an http.Handler that intercepts LLM API traffic.
type Proxy struct {
	cfg      *config.Config
	rp       *httputil.ReverseProxy
	sessions *sessionTracker
}

// New creates a Proxy from the given config.
func New(cfg *config.Config) *Proxy {
	p := &Proxy{
		cfg:      cfg,
		sessions: &sessionTracker{entries: make(map[string]*sessionEntry)},
	}
	p.rp = &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
		Transport:      upstreamTransport(),
	}
	return p
}

// upstreamTransport returns an http.Transport tuned for LLM API calls:
// - explicit dial + TLS timeouts to prevent hung connections
// - generous response-header timeout (models can queue before starting)
// - DisableCompression so our Accept-Encoding: identity in director is respected
func upstreamTransport() http.RoundTripper {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 90 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second, // queued requests can take a while
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		DisableCompression:    true, // we set Accept-Encoding: identity in director
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.rp.ServeHTTP(w, r)
}

// director mutates the outbound request: detects provider, rewrites URL,
// stamps a request ID and start time into the context.
func (p *Proxy) director(req *http.Request) {
	provider, ok := providers.Detect(req)
	if !ok {
		// Anthropic-destined requests that slip through provider detection
		// (OAuth, model listing, any path without anthropic-version header) must
		// still reach the real API. Forward transparently — no usage is captured.
		if req.Header.Get("anthropic-version") != "" || req.Header.Get("x-api-key") != "" {
			base := p.cfg.Proxy.Upstreams["anthropic"]
			if base == "" {
				base = "https://api.anthropic.com"
			}
			if upstream, err := url.Parse(base); err == nil {
				req.URL.Scheme = upstream.Scheme
				req.URL.Host = upstream.Host
				req.Host = upstream.Host
				req.Header.Del("X-Forwarded-For")
				slog.Debug("anthropic passthrough (no provider match)", "path", req.URL.Path)
			}
			return
		}
		slog.Warn("no provider matched",
			"host", req.Host, "path", req.URL.Path,
			"tip", "set ANTHROPIC_BASE_URL or OPENAI_BASE_URL=http://127.0.0.1:4191",
		)
		return
	}

	sessionID := extractSessionID(req)
	if sessionID == "" {
		// No explicit session header — derive a synthetic one so consecutive
		// requests from the same user/client are grouped in the dashboard.
		sessionID = p.sessions.get(systemUsername() + "|" + detectClientName(req))
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, ctxProvider, provider)
	ctx = context.WithValue(ctx, ctxRequestID, uuid.New().String())
	ctx = context.WithValue(ctx, ctxStartTime, time.Now())
	ctx = context.WithValue(ctx, ctxSessionID, sessionID)
	*req = *req.WithContext(ctx)

	configuredBase := p.cfg.Proxy.Upstreams[provider.Name()]
	upstreamRaw := provider.UpstreamURL(req, configuredBase)
	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		slog.Error("invalid upstream URL", "url", upstreamRaw, "err", err)
		return
	}

	req.URL.Scheme = upstream.Scheme
	req.URL.Host = upstream.Host
	req.Host = upstream.Host
	req.Header.Del("X-Forwarded-For")
	// Force uncompressed responses — Go's Transport adds gzip by default,
	// which would make SSE content unparseable.
	req.Header.Set("Accept-Encoding", "identity")

	provider.ModifyRequest(req)
}

// modifyResponse wraps the upstream response body so token usage is captured
// while the response streams to the caller unmodified.
func (p *Proxy) modifyResponse(resp *http.Response) error {
	ctx := resp.Request.Context()

	provider, ok := ctx.Value(ctxProvider).(providers.ProviderPlugin)
	if !ok {
		return nil
	}
	requestID, _ := ctx.Value(ctxRequestID).(string)
	sessionID, _ := ctx.Value(ctxSessionID).(string)
	startTime, _ := ctx.Value(ctxStartTime).(time.Time)

	// Decompress if upstream sent gzip despite our Accept-Encoding: identity.
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err == nil {
			resp.Body = gr
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
		}
	}

	isStream := isSSE(resp)

	clientName, clientVersion := detectClient(resp.Request)
	username := systemUsername()

	onDone := func(model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64) {
		latency := time.Since(startTime).Milliseconds()
		event := providers.UsageEvent{
			RequestID:            requestID,
			SessionID:            sessionID,
			ServiceID:            p.cfg.Proxy.ServiceID,
			Username:             username,
			ClientName:           clientName,
			ClientVersion:        clientVersion,
			Provider:             provider.Name(),
			Model:                model,
			ServiceTier:          serviceTier,
			InferenceGeo:         inferenceGeo,
			TokensInput:          input,
			TokensOutput:         output,
			TokensCached:         cached,
			TokensCachedCreation: cachedCreation,
			LatencyMS:            latency,
			CostUSD:              provider.EstimateCost(model, input, output, cached, cachedCreation),
			Timestamp:            startTime,
			StreamingMode:        isStream,
		}
		p.emit(ctx, event)
	}

	if isStream {
		sp := provider.NewStreamParser()
		resp.Body = newStreamInterceptor(resp.Body, sp, onDone)
	} else {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			return err
		}
		model, tier, geo, input, output, cached, creation, err := provider.ParseResponse(body)
		if err != nil {
			slog.Warn("failed to parse response", "provider", provider.Name(), "err", err)
			return nil
		}
		onDone(model, tier, geo, input, output, cached, creation)
	}

	return nil
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("proxy error", "err", err, "path", r.URL.Path)
	http.Error(w, "upstream error", http.StatusBadGateway)
}

// emit runs the middleware chain then fans the event out to all active sinks.
func (p *Proxy) emit(ctx context.Context, event providers.UsageEvent) {
	event.ServiceID = hashServiceID(event.ServiceID, p.cfg.Privacy.HashServiceID)
	if p.cfg.Privacy.HashUser && event.Username != "" {
		event.Username = hashUser(event.Username, p.cfg.Privacy.OrgSalt)
	}

	slog.Debug("emitting event", "provider", event.Provider, "model", event.Model)

	for _, mw := range middleware.Chain() {
		if err := mw.Process(ctx, &event); err != nil {
			return
		}
	}
	for _, sink := range sinks.All() {
		if err := sink.Write(ctx, event); err != nil {
			slog.Warn("sink write failed", "sink", sink.Name(), "err", err)
		}
	}
}

func isSSE(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream")
}
