package bedrock

import (
	"net/http"
	"testing"
)

func TestDetect(t *testing.T) {
	p := &Plugin{}
	cases := []struct {
		host string
		want bool
	}{
		{"bedrock-runtime.us-east-1.amazonaws.com", true},
		{"bedrock.us-west-2.amazonaws.com", true},
		{"api.anthropic.com", false},
		{"api.openai.com", false},
		{"api.githubcopilot.com", false},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("POST", "https://"+c.host+"/model/anthropic.claude/invoke", nil)
		req.Host = c.host
		if got := p.Detect(req); got != c.want {
			t.Errorf("Detect(%q): got %v, want %v", c.host, got, c.want)
		}
	}
}

func TestParseConverseResponse(t *testing.T) {
	p := &Plugin{}
	body := []byte(`{
		"usage": {"inputTokens": 100, "outputTokens": 50},
		"output": {"message": {"role": "assistant", "content": []}}
	}`)
	_, _, _, input, output, cached, _, err := p.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if input != 100 {
		t.Errorf("input: got %d, want 100", input)
	}
	if output != 50 {
		t.Errorf("output: got %d, want 50", output)
	}
	if cached != 0 {
		t.Errorf("cached: got %d, want 0", cached)
	}
}

func TestStreamParserConverseUsage(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	events := [][]byte{
		[]byte(`{"contentBlockDelta":{"delta":{"text":"Hello"}}}`),
		[]byte(`{"usage":{"inputTokens":80,"outputTokens":20}}`),
	}
	for _, e := range events {
		if err := sp.ConsumeEvent(e); err != nil {
			t.Fatalf("ConsumeEvent: %v", err)
		}
	}
	_, _, _, input, output, _, _ := sp.Result()
	if input != 80 {
		t.Errorf("input: got %d, want 80", input)
	}
	if output != 20 {
		t.Errorf("output: got %d, want 20", output)
	}
}

func TestStreamParserInvocationMetrics(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	// InvokeModelWithResponseStream metadata chunk.
	event := []byte(`{"amazon-bedrock-invocationMetrics":{"inputTokenCount":200,"outputTokenCount":100}}`)
	if err := sp.ConsumeEvent(event); err != nil {
		t.Fatalf("ConsumeEvent: %v", err)
	}
	_, _, _, input, output, _, _ := sp.Result()
	if input != 200 {
		t.Errorf("input: got %d, want 200", input)
	}
	if output != 100 {
		t.Errorf("output: got %d, want 100", output)
	}
}

func TestEstimateCost(t *testing.T) {
	p := &Plugin{}
	// Claude Sonnet on Bedrock: $3/M input, $15/M output.
	cost := p.EstimateCost("anthropic.claude-3-5-sonnet-20241022", 1_000_000, 1_000_000, 0, 0)
	want := 3.00 + 15.00
	if cost != want {
		t.Errorf("cost: got %f, want %f", cost, want)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	p := &Plugin{}
	if cost := p.EstimateCost("unknown-model", 100, 100, 0, 0); cost != 0 {
		t.Errorf("expected 0 cost for unknown model, got %f", cost)
	}
}
