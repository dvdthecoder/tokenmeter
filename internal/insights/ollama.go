package insights

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const systemPrompt = `You are a concise engineering assistant analyzing LLM API cost and usage telemetry. The data contains only token counts, costs, and latency — no prompt or response content. In 3-5 sentences give actionable insights: what stands out, efficiency opportunities, and whether usage looks healthy. Be specific about models and numbers.`

type ollamaReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

type ollamaChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// Generate calls Ollama's /api/generate endpoint and streams the response.
// onToken is called for each token as it arrives (pass nil to suppress).
// Returns the full generated text.
// Returns an error (not a panic) if Ollama is unreachable — callers skip gracefully.
func Generate(ctx context.Context, ollamaURL, model, usageContext string, onToken func(string)) (string, error) {
	payload, _ := json.Marshal(ollamaReq{
		Model:  model,
		Prompt: usageContext,
		System: systemPrompt,
		Stream: true,
	})

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ollamaURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not reachable at %s: %w", ollamaURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ollama: %s — %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk ollamaChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return full.String(), fmt.Errorf("ollama: %s", chunk.Error)
		}
		if chunk.Response != "" {
			full.WriteString(chunk.Response)
			if onToken != nil {
				onToken(chunk.Response)
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("ollama: read stream: %w", err)
	}
	return full.String(), nil
}
