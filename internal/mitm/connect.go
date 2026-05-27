package mitm

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// Handler intercepts HTTPS CONNECT tunnels, terminates TLS with a per-host
// cert signed by the local CA, then hands the decrypted connection to next.
type Handler struct {
	CA   *CA
	Next http.Handler // the normal reverse proxy
}

// ServeHTTP dispatches CONNECT requests to the MITM handler; all others to Next.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r)
		return
	}
	h.Next.ServeHTTP(w, r)
}

func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	hostname := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		hostname = hh
	}

	// Accept the tunnel.
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		slog.Error("mitm: hijack failed", "err", err)
		return
	}
	defer conn.Close()

	// Tell the client the tunnel is ready.
	_, _ = fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")

	if h.CA == nil {
		// No CA — transparent tunnel (no interception).
		h.tunnel(conn, host)
		return
	}

	// MITM: wrap client connection in TLS using our signed cert.
	tlsCfg, err := h.CA.TLSConfigFor(hostname)
	if err != nil {
		slog.Error("mitm: cert generation failed", "host", hostname, "err", err)
		h.tunnel(conn, host)
		return
	}

	clientTLS := tls.Server(conn, tlsCfg)
	if err := clientTLS.Handshake(); err != nil {
		slog.Debug("mitm: TLS handshake failed", "host", hostname, "err", err)
		return
	}
	defer clientTLS.Close()

	// Read decrypted HTTP requests and feed them through the normal proxy.
	br := bufio.NewReader(clientTLS)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			if err != io.EOF {
				slog.Debug("mitm: read request", "host", hostname, "err", err)
			}
			return
		}

		// Reconstruct a proper URL so the proxy director can work.
		req.URL.Scheme = "https"
		req.URL.Host = host
		if req.Host == "" {
			req.Host = host
		}
		req.RequestURI = "" // clear — required for http.ReverseProxy

		rw := &responseWriter{conn: clientTLS}
		h.Next.ServeHTTP(rw, req)
		if err := rw.flush(); err != nil || !rw.keepAlive() {
			return
		}
	}
}

// tunnel is a transparent TCP passthrough — used when CA is nil or cert fails.
func (h *Handler) tunnel(clientConn net.Conn, host string) {
	upstream, err := net.DialTimeout("tcp", host, 15*time.Second)
	if err != nil {
		slog.Error("mitm: tunnel dial failed", "host", host, "err", err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	copy := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go copy(upstream, clientConn)
	go copy(clientConn, upstream)
	<-done
}

// responseWriter captures the proxy response and writes it back to the TLS conn.
// For SSE (text/event-stream) responses it switches to streaming mode: headers are
// sent immediately on WriteHeader, body chunks are written as HTTP/1.1 chunked
// transfer encoding, and the terminal chunk is written in flush(). Non-streaming
// responses use the original buffered path.
type responseWriter struct {
	conn       io.Writer     // *tls.Conn in production; io.Writer for testing
	header     http.Header
	statusCode int
	body       []byte        // non-streaming accumulator
	written    bool
	streaming  bool          // true once Content-Type: text/event-stream detected
	bufW       *bufio.Writer // used only in streaming mode
}

func (rw *responseWriter) Header() http.Header {
	if rw.header == nil {
		rw.header = make(http.Header)
	}
	return rw.header
}

// WriteHeader detects SSE responses and immediately flushes the HTTP status line
// and headers to the wire so streaming can begin. Non-SSE responses are unchanged.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	if strings.Contains(rw.header.Get("Content-Type"), "text/event-stream") {
		rw.streaming = true
		rw.bufW = bufio.NewWriter(rw.conn)
		rw.sendStreamingHeaders(code)
	}
}

// sendStreamingHeaders writes the HTTP status line + headers to the wire with
// Transfer-Encoding: chunked so the body can follow as individual chunks.
func (rw *responseWriter) sendStreamingHeaders(code int) {
	w := rw.bufW
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", code, http.StatusText(code))
	h := rw.header.Clone()
	h.Set("Transfer-Encoding", "chunked")
	h.Del("Content-Length") // incompatible with chunked encoding
	_ = h.Write(w)          // writes canonical "Key: Value\r\n" lines
	fmt.Fprintf(w, "\r\n")  // blank line separating headers from body
	_ = w.Flush()
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.streaming {
		if err := writeHTTPChunk(rw.bufW, b); err != nil {
			return 0, err
		}
		_ = rw.bufW.Flush()
		return len(b), nil
	}
	rw.body = append(rw.body, b...)
	return len(b), nil
}

// Flush implements http.Flusher so httputil.ReverseProxy drives backpressure
// correctly on streaming responses.
func (rw *responseWriter) Flush() {
	if rw.streaming && rw.bufW != nil {
		_ = rw.bufW.Flush()
	}
}

func (rw *responseWriter) flush() error {
	if rw.written {
		return nil
	}
	rw.written = true

	if rw.streaming {
		// Write HTTP/1.1 terminal chunk to signal end of chunked stream.
		_, err := io.WriteString(rw.bufW, "0\r\n\r\n")
		if err != nil {
			return err
		}
		return rw.bufW.Flush()
	}

	// Non-streaming: original buffered path.
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	resp := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: rw.statusCode,
		Status:     fmt.Sprintf("%d %s", rw.statusCode, http.StatusText(rw.statusCode)),
		Header:     rw.header,
		Body:       io.NopCloser(strings.NewReader(string(rw.body))),
	}
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	resp.ContentLength = int64(len(rw.body))
	return resp.Write(rw.conn)
}

func (rw *responseWriter) keepAlive() bool {
	if rw.header == nil {
		return false
	}
	return strings.EqualFold(rw.header.Get("Connection"), "keep-alive")
}

// writeHTTPChunk writes a single HTTP/1.1 chunked transfer encoding frame.
func writeHTTPChunk(w *bufio.Writer, data []byte) error {
	if _, err := fmt.Fprintf(w, "%x\r\n", len(data)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}
