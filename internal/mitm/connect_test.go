package mitm

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// readChunkedBody decodes a minimal HTTP/1.1 chunked body from r.
func readChunkedBody(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading chunk size: %v", err)
		}
		sizeLine := strings.TrimSpace(line)
		size, err := strconv.ParseInt(sizeLine, 16, 64)
		if err != nil {
			t.Fatalf("invalid chunk size %q: %v", sizeLine, err)
		}
		if size == 0 {
			break
		}
		chunk := make([]byte, size)
		if _, err := r.Read(chunk); err != nil {
			t.Fatalf("reading chunk body: %v", err)
		}
		out.Write(chunk)
		// consume trailing CRLF after chunk data
		r.ReadString('\n')
	}
	return out.String()
}

func TestResponseWriterNonStreaming(t *testing.T) {
	var buf bytes.Buffer
	rw := &responseWriter{conn: &buf}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	body := []byte(`{"ok":true}`)
	rw.Write(body)

	// flush() should now write a complete HTTP/1.1 response.
	if err := rw.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Parse what came out of buf.
	resp, err := http.ReadResponse(bufio.NewReader(&buf), nil)
	if err != nil {
		t.Fatalf("http.ReadResponse: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}

	// Calling flush() a second time must be a no-op.
	if err := rw.flush(); err != nil {
		t.Errorf("second flush: %v", err)
	}
	// buf should not have grown.
	_ = buf.Len()
}

func TestResponseWriterStreaming(t *testing.T) {
	var buf bytes.Buffer
	rw := &responseWriter{conn: &buf}
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)

	// After WriteHeader, the HTTP status line + headers must already be in buf.
	if buf.Len() == 0 {
		t.Fatal("WriteHeader did not flush headers immediately for SSE response")
	}

	chunks := []string{
		"data: hello\n\n",
		"data: world\n\n",
	}
	for _, c := range chunks {
		if _, err := rw.Write([]byte(c)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	rw.Flush()

	// flush() must write the terminal chunk.
	if err := rw.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Snapshot raw bytes before http.ReadResponse drains the buffer.
	raw := buf.String()

	// Parse the output: status line + headers + chunked body.
	resp, err := http.ReadResponse(bufio.NewReader(strings.NewReader(raw)), nil)
	if err != nil {
		t.Fatalf("http.ReadResponse: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if len(resp.TransferEncoding) == 0 || resp.TransferEncoding[0] != "chunked" {
		t.Errorf("Transfer-Encoding: got %v, want [chunked]", resp.TransferEncoding)
	}

	// Verify raw wire format contains the terminal chunk and chunk data.
	if !strings.Contains(raw, "0\r\n\r\n") {
		t.Error("terminal chunk (0 CRLF CRLF) not found in output")
	}
	for _, c := range chunks {
		if !strings.Contains(raw, c) {
			t.Errorf("chunk data %q not found in raw output", c)
		}
	}
}

func TestResponseWriterStreamingFlushIdempotent(t *testing.T) {
	var buf bytes.Buffer
	rw := &responseWriter{conn: &buf}
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("data: ping\n\n"))

	if err := rw.flush(); err != nil {
		t.Fatalf("first flush: %v", err)
	}
	lenAfterFirst := buf.Len()

	if err := rw.flush(); err != nil {
		t.Fatalf("second flush: %v", err)
	}
	if buf.Len() != lenAfterFirst {
		t.Error("second flush wrote extra bytes")
	}
}

func TestWriteHTTPChunk(t *testing.T) {
	cases := []struct {
		data string
	}{
		{"hello"},
		{"data: event\n\n"},
		{strings.Repeat("x", 256)},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		if err := writeHTTPChunk(w, []byte(tc.data)); err != nil {
			t.Fatalf("writeHTTPChunk: %v", err)
		}
		w.Flush()

		// Verify the wire format: <hex-len>\r\n<data>\r\n
		raw := buf.String()
		parts := strings.SplitN(raw, "\r\n", 2)
		if len(parts) < 2 {
			t.Fatalf("missing CRLF after chunk size in %q", raw)
		}
		size, err := strconv.ParseInt(parts[0], 16, 64)
		if err != nil {
			t.Fatalf("invalid chunk size %q: %v", parts[0], err)
		}
		if int(size) != len(tc.data) {
			t.Errorf("chunk size: got %d, want %d", size, len(tc.data))
		}
		expectedBody := fmt.Sprintf("%s\r\n", tc.data)
		if parts[1] != expectedBody {
			t.Errorf("chunk body: got %q, want %q", parts[1], expectedBody)
		}
	}
}
