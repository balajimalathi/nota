package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nota/internal/embed"
)

func TestStreamEmbedAndPrompts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Write([]byte(`{"data":[]}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		case "/v1/embeddings":
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0]},{"index":1,"embedding":[0,1]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/v1", "k", "m", "e")
	if err := c.Reachable(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	err := c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(s string) {
		got.WriteString(s)
	})
	if err != nil || got.String() != "Hello there" {
		t.Fatalf("%v %q", err, got.String())
	}
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil || len(vecs) != 2 || vecs[1][1] != 1 {
		t.Fatalf("%v %#v", err, vecs)
	}
	msgs := AskMessages("why?", []embed.Hit{{Path: "a.md", Heading: "H", Text: "because"}})
	if len(msgs) != 2 || !strings.Contains(msgs[1].Content, "[[") && !strings.Contains(msgs[0].Content, "[[") {
		t.Fatalf("ask %#v", msgs)
	}
	if !strings.Contains(msgs[1].Content, "a.md") || !strings.Contains(msgs[0].Content, "[[Title]]") {
		t.Fatalf("citations missing: %#v", msgs)
	}
	title := CommandMessages("title", "body", "")
	if !strings.Contains(title[1].Content, "JSON") {
		t.Fatal(title[1].Content)
	}
}
