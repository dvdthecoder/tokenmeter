package proxy

import (
	"bytes"
	"io"
	"log/slog"

	"github.com/yourorg/tokenmeter/plugins/providers"
)

// maxBufSize caps the line-assembly buffer. SSE data lines are never large;
// anything bigger indicates a malformed or adversarial stream.
const maxBufSize = 1 << 20 // 1 MB

// streamInterceptor wraps an SSE response body.
// It forwards every byte to the caller immediately while feeding each
// "data: ..." line to the StreamParser. On EOF it fires onDone.
type streamInterceptor struct {
	src    io.ReadCloser
	parser providers.StreamParser
	onDone func(model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64)
	buf    []byte
}

func newStreamInterceptor(
	src io.ReadCloser,
	parser providers.StreamParser,
	onDone func(model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64),
) io.ReadCloser {
	return &streamInterceptor{src: src, parser: parser, onDone: onDone}
}

func (s *streamInterceptor) Read(p []byte) (int, error) {
	n, err := s.src.Read(p)
	if n > 0 {
		s.processChunk(p[:n])
	}
	if err == io.EOF {
		// Flush any remaining partial line.
		if len(s.buf) > 0 {
			s.consumeLine(s.buf)
			s.buf = nil
		}
		model, tier, geo, input, output, cached, creation := s.parser.Result()
		s.onDone(model, tier, geo, input, output, cached, creation)
	}
	return n, err
}

func (s *streamInterceptor) Close() error {
	return s.src.Close()
}

// processChunk splits the incoming bytes into lines and feeds data: lines
// to the parser. Only data: lines are inspected — content is never retained.
func (s *streamInterceptor) processChunk(data []byte) {
	s.buf = append(s.buf, data...)
	if len(s.buf) > maxBufSize {
		slog.Warn("stream buffer overflow — discarding; stream may be malformed",
			"size", len(s.buf))
		s.buf = nil
	}
	for {
		idx := bytes.IndexByte(s.buf, '\n')
		if idx < 0 {
			break
		}
		line := bytes.TrimRight(s.buf[:idx], "\r")
		s.buf = s.buf[idx+1:]
		s.consumeLine(line)
	}
}

func (s *streamInterceptor) consumeLine(line []byte) {
	if !bytes.HasPrefix(line, []byte("data: ")) {
		return
	}
	payload := line[6:]
	if bytes.Equal(payload, []byte("[DONE]")) {
		return
	}
	_ = s.parser.ConsumeEvent(payload)
}
