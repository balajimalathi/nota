package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"nota/internal/ai"
	"nota/internal/embed"
)

func (app *App) aiPane(c *ui.Context) {
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		ui.Text(c, "Local model · "+app.aiReach).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(4).Children(func() {
			if ui.Button(c, "Summarize").Clicked() {
				app.runAI("summarize")
			}
			if ui.Button(c, "Rewrite").Clicked() {
				app.runAI("rewrite")
			}
			if ui.Button(c, "Continue").Clicked() {
				app.runAI("continue")
			}
			if ui.Button(c, "Title").Clicked() {
				app.runAI("title")
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				if len(app.aiMessages) == 0 {
					ui.Text(c, "Ask about this note, or index the vault and ask across it. Works with Ollama at the endpoint in Settings.").TextColor(c.Theme().TextMuted)
				}
				for i := range app.aiMessages {
					m := app.aiMessages[i]
					who := "You"
					if m.Role == "assistant" {
						who = "Nota"
					}
					ui.Text(c, who).Bold()
					if m.Role == "assistant" && m.Done {
						markdown.View(c, m.Text, &app.mdOpts)
					} else if m.Role == "assistant" {
						markdown.Stream(c, m.Text, &app.mdOpts)
					} else {
						ui.Text(c, m.Text)
					}
				}
			})
		})
		ui.Row(c).Gap(6).Children(func() {
			if ui.Button(c, "Insert").Clicked() {
				app.insertAI()
			}
			if ui.Button(c, "Index vault").Clicked() {
				app.indexVault()
			}
		})
		if ui.TextInput(c, &app.aiInput).Placeholder("Ask the vault…").Submitted() {
			app.askVault()
		}
	})
}

func (app *App) runAI(kind string) {
	if app.aiBusy || app.client == nil {
		return
	}
	t := app.current()
	noteText := ""
	sel := ""
	if t != nil {
		noteText = t.state.Buffer.String()
		sel = t.state.SelectedText()
	}
	label := kind
	switch kind {
	case "summarize":
		label = "Summarize this note"
	case "rewrite":
		label = "Rewrite"
	case "continue":
		label = "Continue writing"
	case "title":
		label = "Suggest a title and tags"
	}
	app.stream(label, ai.CommandMessages(kind, noteText, sel), kind == "title")
}

func (app *App) askVault() {
	if app.aiBusy || app.client == nil {
		return
	}
	q := strings.TrimSpace(app.aiInput)
	if q == "" {
		app.status = "Type a question first"
		return
	}
	app.aiInput = ""
	app.aiBusy = true
	go func() {
		vecs, err := app.client.Embed(context.Background(), []string{q})
		if err != nil {
			app.failAI(err)
			return
		}
		if len(vecs) == 0 {
			app.failAI(errEmpty())
			return
		}
		hits := app.store.Search(vecs[0], 6, "")
		msgs := ai.AskMessages(q, hits)
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			app.aiBusy = false
			if len(hits) == 0 {
				app.status = "No embeddings yet. Use Index vault."
			}
			app.stream("Ask: "+q, msgs, false)
		})
	}()
}

func (app *App) findRelated() {
	t := app.current()
	if t == nil || app.client == nil || app.aiBusy {
		return
	}
	text := t.state.Buffer.String()
	if len(text) > 2000 {
		text = text[:2000]
	}
	path := t.path
	go func() {
		vecs, err := app.client.Embed(context.Background(), []string{text})
		if err != nil {
			app.failAI(err)
			return
		}
		hits := app.store.Search(vecs[0], 6, path)
		if app.win == nil {
			return
		}
		app.win.Update(func() { app.related = hits })
	}()
}

func (app *App) indexVault() {
	if app.vault == nil || app.aiBusy || app.client == nil {
		return
	}
	app.aiBusy = true
	app.status = "Indexing vectors…"
	root := app.vault.Root()
	paths := app.index.Paths()
	go func() {
		var missing []embed.Chunk
		for _, p := range paths {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			if err != nil {
				continue
			}
			for _, c := range embed.Chunks(p, string(b)) {
				if !app.store.Has(c.Hash) {
					missing = append(missing, c)
				}
			}
		}
		const batch = 16
		for i := 0; i < len(missing); i += batch {
			end := i + batch
			if end > len(missing) {
				end = len(missing)
			}
			part := missing[i:end]
			in := make([]string, len(part))
			for j, c := range part {
				in[j] = c.Heading + "\n" + c.Text
			}
			vecs, err := app.client.Embed(context.Background(), in)
			if err != nil {
				app.failAI(err)
				return
			}
			if err := app.store.Upsert(part, vecs); err != nil {
				app.failAI(err)
				return
			}
		}
		if app.vault != nil {
			if dir, err := app.vault.EnsureApp(); err == nil {
				_ = app.store.Save(filepath.Join(dir, "embeddings.bin"))
			}
		}
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			app.aiBusy = false
			app.aiReach = "ready"
			app.status = "Embedded " + itoa(len(missing)) + " new chunks"
		})
	}()
}

func (app *App) stream(label string, msgs []ai.Message, titleJSON bool) {
	if app.aiBusy {
		return
	}
	app.aiBusy = true
	app.showRight = true
	app.rightTab = 4
	app.aiMessages = append(app.aiMessages, chatMsg{Role: "user", Text: label, Done: true})
	app.aiMessages = append(app.aiMessages, chatMsg{Role: "assistant"})
	idx := len(app.aiMessages) - 1
	go func() {
		err := app.client.Stream(context.Background(), msgs, func(s string) {
			if app.win == nil {
				return
			}
			app.win.Update(func() {
				if idx < len(app.aiMessages) {
					app.aiMessages[idx].Text += s
				}
			})
		})
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			app.aiBusy = false
			if idx < len(app.aiMessages) {
				app.aiMessages[idx].Done = true
				if err != nil {
					app.aiMessages[idx].Text += "\n\n" + err.Error()
					app.aiReach = "offline"
				} else if titleJSON {
					app.applyTitle(app.aiMessages[idx].Text)
				}
			}
		})
	}()
}

func (app *App) applyTitle(text string) {
	t := app.current()
	if t == nil {
		return
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return
	}
	var parsed struct {
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if json.Unmarshal([]byte(text[start:end+1]), &parsed) != nil || parsed.Title == "" {
		return
	}
	src := t.state.Buffer.String()
	next := putFrontmatter(src, parsed.Title, parsed.Tags)
	if next == src {
		return
	}
	t.state.Buffer.Reset(next)
	app.status = "Updated title and tags"
}

func putFrontmatter(src, title string, tags []string) string {
	block := "title: " + title + "\n"
	if len(tags) > 0 {
		block += "tags:\n"
		for _, tag := range tags {
			tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
			if tag != "" {
				block += "  - " + tag + "\n"
			}
		}
	}
	if strings.HasPrefix(src, "---\n") || strings.HasPrefix(src, "---\r\n") {
		rest := src
		nl := strings.IndexByte(rest, '\n')
		if nl < 0 {
			return "---\n" + block + "---\n\n" + src
		}
		search := rest[nl+1:]
		off := nl + 1
		for {
			lineEnd := strings.IndexByte(search, '\n')
			line := search
			if lineEnd >= 0 {
				line = search[:lineEnd]
			}
			if strings.TrimSpace(strings.TrimRight(line, "\r")) == "---" {
				end := off + len(line)
				if lineEnd >= 0 {
					end = off + lineEnd + 1
				}
				return "---\n" + block + "---\n" + src[end:]
			}
			if lineEnd < 0 {
				break
			}
			search = search[lineEnd+1:]
			off += lineEnd + 1
		}
	}
	return "---\n" + block + "---\n\n" + src
}

func (app *App) insertAI() {
	t := app.current()
	if t == nil {
		return
	}
	for i := len(app.aiMessages) - 1; i >= 0; i-- {
		if app.aiMessages[i].Role == "assistant" && strings.TrimSpace(app.aiMessages[i].Text) != "" {
			t.state.Insert("\n" + app.aiMessages[i].Text)
			return
		}
	}
}

func (app *App) failAI(err error) {
	if app.win == nil || err == nil {
		return
	}
	app.win.Update(func() {
		app.aiBusy = false
		app.aiReach = "offline"
		app.status = err.Error()
	})
}

type emptyErr struct{}

func (emptyErr) Error() string { return "no embedding returned" }
func errEmpty() error          { return emptyErr{} }
