# Nota

A small, offline Markdown vault. The window is native (MyGo, the same UI toolkit as [Mayoi](https://github.com/sorrycc/Mayoi)), notes are plain files, and AI is optional.

## Run

```sh
go run ./cmd/nota
go run ./cmd/nota C:\path\to\vault
go build -o build/nota.exe ./cmd/nota
```

Needs Go 1.27 or later. The first build downloads MyGo.

A vault is a folder of `.md` files. Wikilinks (`[[Note]]`, `[[Note|label]]`, `![[image.png]]`), tags, highlights (`==text==`), tasks, callouts, and YAML frontmatter (`title`, `aliases`, `tags`) are indexed. Caches live in `.app/` and can be deleted.

## AI

Point Settings at any OpenAI-compatible server. The default is Ollama:

```sh
ollama pull llama3.2
ollama pull nomic-embed-text
ollama serve
```

Summarize, rewrite, continue, and title/tag run against the open note. **Index vault** embeds notes locally, then **Ask the vault** answers with `[[wikilinks]]`. If nothing is listening, the app still edits notes.

## Sync

Notes stay files. In Sync, pick one of:

- **git** — `git init` in the vault, then pull (fast-forward, or a `note (conflict device date).md` copy when both sides changed) and push.
- **folder** — another directory. The last synced hashes are `.app/sync-base.json`.
- **http** — `nota serve -vault <dir> -addr 127.0.0.1:8765 -token secret` on the other machine.

## Keys

Defaults are in `%AppData%\nota\keybindings.json` (VS Code’s format, comments allowed). `ctrl` is Ctrl on Windows and Command on macOS.

| Key | Action |
|---|---|
| Ctrl+O | Quick open |
| Ctrl+P | Commands |
| Ctrl+Shift+F | Search |
| Ctrl+N | New note |
| Ctrl+S | Save |
| Ctrl+W | Close tab |
| Ctrl+E | Source / live / reading |
| Ctrl+Shift+D | Daily note |
