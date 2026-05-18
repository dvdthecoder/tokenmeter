package gemini

import (
	"net/http"
	"testing"
)

// --- Stream parser tests ---

var streamFixture = []string{
	// Intermediate chunks — no usageMetadata yet
	`{"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}],"modelVersion":"gemini-2.0-flash"}`,
	`{"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"}}]}`,
	// Final chunk — authoritative usageMetadata
	`{"candidates":[{"content":{"parts":[{"text":"!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":142,"candidatesTokenCount":75,"cachedContentTokenCount":30},"modelVersion":"gemini-2.0-flash"}`,
}

func TestStreamParser(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	for _, line := range streamFixture {
		if err := sp.ConsumeEvent([]byte(line)); err != nil {
			t.Fatalf("ConsumeEvent: %v", err)
		}
	}

	model, _, _, input, output, cached, creation := sp.Result()

	if model != "gemini-2.0-flash" {
		t.Errorf("model: got %q, want %q", model, "gemini-2.0-flash")
	}
	if input != 142 {
		t.Errorf("input: got %d, want 142", input)
	}
	if output != 75 {
		t.Errorf("output: got %d, want 75", output)
	}
	if cached != 30 {
		t.Errorf("cached: got %d, want 30", cached)
	}
	if creation != 0 {
		t.Errorf("cachedCreation: got %d, want 0 (Gemini has no cache write tokens)", creation)
	}
}

func TestStreamParserFinalChunkWins(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	// Intermediate chunk with partial (wrong) counts
	_ = sp.ConsumeEvent([]byte(`{"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5},"modelVersion":"gemini-2.0-flash"}`))
	// Final chunk with correct totals
	_ = sp.ConsumeEvent([]byte(`{"usageMetadata":{"promptTokenCount":142,"candidatesTokenCount":75},"modelVersion":"gemini-2.0-flash"}`))

	_, _, _, input, output, _, _ := sp.Result()
	if input != 142 {
		t.Errorf("final chunk should win: input got %d, want 142", input)
	}
	if output != 75 {
		t.Errorf("final chunk should win: output got %d, want 75", output)
	}
}

func TestStreamParserMalformedChunkIgnored(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()
	if err := sp.ConsumeEvent([]byte(`not json`)); err != nil {
		t.Errorf("malformed chunk should be silently ignored, got: %v", err)
	}
}

// --- ParseResponse (non-streaming) ---

var nonStreamFixture = []byte(`{
  "candidates": [{"content":{"parts":[{"text":"Hi"}],"role":"model"},"finishReason":"STOP"}],
  "usageMetadata": {
    "promptTokenCount": 20,
    "candidatesTokenCount": 5,
    "cachedContentTokenCount": 0
  },
  "modelVersion": "gemini-1.5-flash"
}`)

func TestParseResponse(t *testing.T) {
	p := &Plugin{}
	model, _, _, input, output, cached, creation, err := p.ParseResponse(nonStreamFixture)
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemini-1.5-flash" {
		t.Errorf("model: got %q", model)
	}
	if input != 20 {
		t.Errorf("input: got %d, want 20", input)
	}
	if output != 5 {
		t.Errorf("output: got %d, want 5", output)
	}
	if cached != 0 {
		t.Errorf("cached: got %d, want 0", cached)
	}
	if creation != 0 {
		t.Errorf("cachedCreation: got %d, want 0", creation)
	}
}

// --- Detect ---

func TestDetect(t *testing.T) {
	p := &Plugin{}

	cases := []struct {
		host string
		want bool
	}{
		{"generativelanguage.googleapis.com", true},
		{"api.anthropic.com", false},
		{"api.openai.com", false},
		{"generativelanguage.googleapis.com:443", true},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("POST", "https://"+c.host+"/v1beta/models/gemini-2.0-flash:generateContent", nil)
		req.Host = c.host
		got := p.Detect(req)
		if got != c.want {
			t.Errorf("Detect(%q): got %v, want %v", c.host, got, c.want)
		}
	}
}

// --- EstimateCost ---

func TestEstimateCost(t *testing.T) {
	p := &Plugin{}

	// gemini-2.0-flash: $0.10/$0.40 per 1M
	// 1M input + 1M output = $0.10 + $0.40 = $0.50
	cost := p.EstimateCost("gemini-2.0-flash", 1_000_000, 1_000_000, 0, 0)
	if cost < 0.49 || cost > 0.51 {
		t.Errorf("cost for 1M+1M gemini-2.0-flash: got %f, want ~0.50", cost)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	p := &Plugin{}
	// Should not panic — falls back to flash pricing
	cost := p.EstimateCost("gemini-unknown-future-model", 1_000_000, 0, 0, 0)
	if cost <= 0 {
		t.Error("unknown model should still return a positive cost estimate")
	}
}

func TestEstimateCostCachedTokens(t *testing.T) {
	p := &Plugin{}
	// Cached at 25% of input price: 1M cached * $0.10 * 0.25 = $0.025
	cost := p.EstimateCost("gemini-2.0-flash", 0, 0, 1_000_000, 0)
	if cost < 0.024 || cost > 0.026 {
		t.Errorf("cached token cost: got %f, want ~0.025", cost)
	}
}
