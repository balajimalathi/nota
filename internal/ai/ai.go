// Package ai talks to a local OpenAI-compatible server, such as Ollama.
package ai

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

	"nota/internal/embed"
)

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client calls /v1/chat/completions and /v1/embeddings.
type Client struct {
	BaseURL    string
	APIKey     string
	ChatModel  string
	EmbedModel string
	HTTP       *http.Client
}

// New returns a client. Empty values fall back to a local Ollama.
func New(base, key, chat, embedModel string) *Client {
	if strings.TrimSpace(base) == "" {
		base = "http://localhost:11434/v1"
	}
	if chat == "" {
		chat = "llama3.2"
	}
	if embedModel == "" {
		embedModel = "nomic-embed-text"
	}
	return &Client{
		BaseURL:    strings.TrimRight(base, "/"),
		APIKey:     key,
		ChatModel:  chat,
		EmbedModel: embedModel,
		HTTP:       &http.Client{Timeout: 0},
	}
}

// Reachable reports whether the server answers /v1/models.
func (c *Client) Reachable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	c.auth(req)
	res, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("model server returned %s", res.Status)
	}
	return nil
}

// Stream sends a chat completion and calls onDelta with each text piece.
func (c *Client) Stream(ctx context.Context, msgs []Message, onDelta func(string)) error {
	body, err := json.Marshal(map[string]any{
		"model":    c.ChatModel,
		"messages": msgs,
		"stream":   true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	res, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("chat: %s: %s", res.Status, bytes.TrimSpace(b))
	}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		line = strings.TrimPrefix(line, "data:")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("%s", chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			text := ch.Delta.Content
			if text == "" {
				text = ch.Message.Content
			}
			if text != "" && onDelta != nil {
				onDelta(text)
			}
		}
	}
	return sc.Err()
}

// Embed returns one vector per input string.
func (c *Client) Embed(ctx context.Context, input []string) ([][]float32, error) {
	if len(input) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"model": c.EmbedModel,
		"input": input,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	res, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings: %s: %s", res.Status, bytes.TrimSpace(b))
	}
	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	out := make([][]float32, len(input))
	for _, row := range parsed.Data {
		if row.Index >= 0 && row.Index < len(out) {
			out[row.Index] = row.Embedding
		}
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("missing embedding %d", i)
		}
	}
	return out, nil
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) auth(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

// AskMessages builds a grounded question over retrieved chunks.
func AskMessages(question string, hits []embed.Hit) []Message {
	var b strings.Builder
	b.WriteString("Notes:\n\n")
	for _, h := range hits {
		fmt.Fprintf(&b, "Note: %s\n", h.Path)
		if h.Heading != "" {
			fmt.Fprintf(&b, "Heading: %s\n", h.Heading)
		}
		b.WriteString(h.Text)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Question: %s", question)
	return []Message{
		{Role: "system", Content: "You answer from the notes below. Cite notes as [[Title]]. If the notes do not say, say so. Be concise."},
		{Role: "user", Content: b.String()},
	}
}

// CommandMessages builds a prompt for summarize, rewrite, continue or title.
func CommandMessages(kind, note, selection string) []Message {
	note = clip(note, 12000)
	selection = clip(selection, 6000)
	var user string
	switch kind {
	case "summarize":
		user = "Summarize this note in a short paragraph:\n\n" + note
	case "rewrite":
		src := selection
		if strings.TrimSpace(src) == "" {
			src = note
		}
		user = "Rewrite the following so it is clearer, preserving its meaning. Return only the rewrite:\n\n" + src
	case "continue":
		user = "Continue this note in the same voice. Return only the continuation:\n\n" + note
	case "title":
		user = "Suggest a title and up to four tags for this note. Reply as JSON {\"title\":\"\",\"tags\":[]} and nothing else:\n\n" + note
	default:
		user = note
	}
	return []Message{
		{Role: "system", Content: "You help edit a local Markdown vault. Do not invent citations."},
		{Role: "user", Content: user},
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
