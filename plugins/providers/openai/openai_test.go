package openai

import (
	"testing"
)

// Fixture: streaming response with include_usage=true (final chunk carries usage).
var streamFixture = []string{
	`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
	`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
	`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":20}}}`,
}

// vLLM fixture — model name is the local model identifier.
var vllmStreamFixture = []string{
	`{"id":"cmpl-1","object":"chat.completion.chunk","model":"qwen2.5-coder-32b","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
	`{"id":"cmpl-1","object":"chat.completion.chunk","model":"qwen2.5-coder-32b","choices":[{"index":0,"delta":{"content":"fn main"},"finish_reason":null}]}`,
	`{"id":"cmpl-1","object":"chat.completion.chunk","model":"qwen2.5-coder-32b","choices":[],"usage":{"prompt_tokens":200,"completion_tokens":80,"total_tokens":280}}`,
}

var nonStreamFixture = []byte(`{
	"id":"chatcmpl-2","object":"chat.completion","model":"gpt-4o-mini",
	"choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],
	"usage":{"prompt_tokens":15,"completion_tokens":3,"total_tokens":18,"prompt_tokens_details":{"cached_tokens":10}}
}`)

func TestStreamParser(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	for _, line := range streamFixture {
		if err := sp.ConsumeEvent([]byte(line)); err != nil {
			t.Fatalf("ConsumeEvent error: %v", err)
		}
	}

	model, _, _, input, output, cached, _ := sp.Result()

	if model != "gpt-4o" {
		t.Errorf("model: got %q, want gpt-4o", model)
	}
	if input != 100 {
		t.Errorf("input tokens: got %d, want 100", input)
	}
	if output != 50 {
		t.Errorf("output tokens: got %d, want 50", output)
	}
	if cached != 20 {
		t.Errorf("cached tokens: got %d, want 20", cached)
	}
}

func TestVLLMStreamParser(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	for _, line := range vllmStreamFixture {
		_ = sp.ConsumeEvent([]byte(line))
	}

	model, _, _, input, output, cached, _ := sp.Result()

	if model != "qwen2.5-coder-32b" {
		t.Errorf("model: got %q", model)
	}
	if input != 200 || output != 80 || cached != 0 {
		t.Errorf("tokens: in=%d out=%d cached=%d", input, output, cached)
	}
}

func TestParseResponse(t *testing.T) {
	p := &Plugin{}
	model, _, _, input, output, cached, _, err := p.ParseResponse(nonStreamFixture)
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-4o-mini" {
		t.Errorf("model: got %q", model)
	}
	if input != 15 || output != 3 || cached != 10 {
		t.Errorf("tokens: in=%d out=%d cached=%d", input, output, cached)
	}
}

func TestEstimateCost(t *testing.T) {
	p := &Plugin{}
	// 1M prompt + 1M completion on gpt-4o = $2.50 + $10.00 = $12.50
	cost := p.EstimateCost("gpt-4o", 1_000_000, 1_000_000, 0, 0)
	if cost != 12.50 {
		t.Errorf("cost: got %.4f, want 12.50", cost)
	}
}

func TestVLLMZeroCost(t *testing.T) {
	p := &Plugin{}
	cost := p.EstimateCost("qwen2.5-coder-32b", 1_000_000, 1_000_000, 0, 0)
	if cost != 0 {
		t.Errorf("vLLM cost should be 0, got %.4f", cost)
	}
}

func TestUnknownModelZeroCost(t *testing.T) {
	p := &Plugin{}
	cost := p.EstimateCost("some-unknown-model", 1_000_000, 1_000_000, 0, 0)
	if cost != 0 {
		t.Errorf("unknown model cost should be 0, got %.4f", cost)
	}
}
