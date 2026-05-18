package anthropic

import (
	"testing"
)

var streamFixture = []string{
	`{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":142,"cache_creation_input_tokens":500,"cache_read_input_tokens":300}}}`,
	`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
	`{"type":"content_block_stop","index":0}`,
	`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":75}}`,
	`{"type":"message_stop"}`,
}

var nonStreamFixture = []byte(`{
	"id":"msg_01","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001",
	"content":[{"type":"text","text":"Hi"}],
	"usage":{"input_tokens":20,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}
}`)

func TestStreamParser(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	for _, line := range streamFixture {
		if err := sp.ConsumeEvent([]byte(line)); err != nil {
			t.Fatalf("ConsumeEvent error: %v", err)
		}
	}

	model, _, _, input, output, cached, creation := sp.Result()

	if model != "claude-sonnet-4-6" {
		t.Errorf("model: got %q, want %q", model, "claude-sonnet-4-6")
	}
	if input != 142 {
		t.Errorf("input tokens: got %d, want 142", input)
	}
	if output != 75 {
		t.Errorf("output tokens: got %d, want 75", output)
	}
	if cached != 300 {
		t.Errorf("cached tokens: got %d, want 300", cached)
	}
	if creation != 500 {
		t.Errorf("cache creation tokens: got %d, want 500", creation)
	}
}

func TestParseResponse(t *testing.T) {
	p := &Plugin{}
	model, _, _, input, output, cached, creation, err := p.ParseResponse(nonStreamFixture)
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-haiku-4-5-20251001" {
		t.Errorf("model: got %q", model)
	}
	if input != 20 || output != 5 || cached != 0 || creation != 0 {
		t.Errorf("tokens: got in=%d out=%d cached=%d creation=%d", input, output, cached, creation)
	}
}

func TestEstimateCost(t *testing.T) {
	p := &Plugin{}
	// 1M input + 1M output on Sonnet = $3.00 + $15.00 = $18.00
	cost := p.EstimateCost("claude-sonnet-4-6", 1_000_000, 1_000_000, 0, 0)
	if cost != 18.00 {
		t.Errorf("cost: got %.4f, want 18.00", cost)
	}
}

func TestUnknownEventTypesIgnored(t *testing.T) {
	p := &Plugin{}
	sp := p.NewStreamParser()

	unknowns := []string{
		`{"type":"ping"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ignored content"}}`,
		`{"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
	}
	for _, line := range unknowns {
		_ = sp.ConsumeEvent([]byte(line))
	}
	_, _, _, input, output, _, _ := sp.Result()
	if input != 10 || output != 5 {
		t.Errorf("unexpected counts: in=%d out=%d", input, output)
	}
}
