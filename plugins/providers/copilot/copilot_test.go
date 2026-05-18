package copilot

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
		{"api.githubcopilot.com", true},
		{"api.githubcopilot.com:443", true},
		{"api.openai.com", false},
		{"api.anthropic.com", false},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("POST", "https://"+c.host+"/chat/completions", nil)
		req.Host = c.host
		if got := p.Detect(req); got != c.want {
			t.Errorf("Detect(%q): got %v, want %v", c.host, got, c.want)
		}
	}
}

func TestEstimateCostIsZero(t *testing.T) {
	p := &Plugin{}
	// Copilot is subscription-based — cost is always 0.
	if cost := p.EstimateCost("gpt-4o", 1_000_000, 1_000_000, 0, 0); cost != 0 {
		t.Errorf("expected 0 cost for Copilot, got %f", cost)
	}
}

var streamFixture = []string{
	`data: {"id":"cmpl-1","choices":[{"delta":{"content":"Hello"}}]}`,
	`data: {"id":"cmpl-1","choices":[{"delta":{"content":" world"}}]}`,
	`data: {"id":"cmpl-1","choices":[{"finish_reason":"stop","delta":{}}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`,
	`data: [DONE]`,
}

func TestStreamParser(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	for _, line := range streamFixture {
		data := []byte(line)
		if len(data) > 6 && string(data[:6]) == "data: " {
			data = data[6:]
		}
		if string(data) == "[DONE]" {
			continue
		}
		if err := sp.ConsumeEvent(data); err != nil {
			t.Fatalf("ConsumeEvent: %v", err)
		}
	}

	_, _, _, input, output, _, _ := sp.Result()
	if input != 20 {
		t.Errorf("input: got %d, want 20", input)
	}
	if output != 10 {
		t.Errorf("output: got %d, want 10", output)
	}
}

func TestParseResponse(t *testing.T) {
	p := &Plugin{}
	body := []byte(`{
		"model": "gpt-4o",
		"usage": {"prompt_tokens": 50, "completion_tokens": 25, "total_tokens": 75}
	}`)
	model, _, _, input, output, _, _, err := p.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-4o" {
		t.Errorf("model: got %q, want gpt-4o", model)
	}
	if input != 50 {
		t.Errorf("input: got %d, want 50", input)
	}
	if output != 25 {
		t.Errorf("output: got %d, want 25", output)
	}
}
