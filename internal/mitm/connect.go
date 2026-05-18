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
type responseWriter struct {
	conn       *tls.Conn
	header     http.Header
	statusCode int
	body       []byte
	written    bool
}

func (rw *responseWriter) Header() http.Header {
	if rw.header == nil {
		rw.header = make(http.Header)
	}
	return rw.header
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return len(b), nil
}

func (rw *responseWriter) flush() error {
	if rw.written {
		return nil
	}
	rw.written = true
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
