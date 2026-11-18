# SEO CMS — Decision log

**Date:** 2026-11-18  
**Status:** Locked (product owner)  
**Source:** [Discovery audit](2026-11-18-repository-discovery-architecture-audit.md) + owner decisions 2026-11-18

| ID | Decision | Resolution |
|----|----------|------------|
| D1 | How WordPress sync lives relative to Nota | **Merge `obsidian-wp-sync` into Nota.** No separate CLI or Obsidian plugin. Port/reuse core sync logic inside the Nota app. **Obsidian is retired as an editor**; Markdown on disk stays **Obsidian-compatible** (plain `.md`, YAML frontmatter, wikilinks) so vaults remain portable. |
| D2 | Docs / site framework content SoT | **Markdown in the Nota site folder is the standard.** Copying into Astro or Next.js is **manual**. Nota’s job is fresh, connected, keyword-rich content—tracking keywords on articles and improving them—not auto-deploying to the framework repo. |
| D3 | WP vs site static duplication | **Keep WordPress URLs and site static content in separate trees.** If the public site serves the **same** content from WordPress, **do not** also keep an editable copy under site static—**WP tree only**. Static holds only non-WP routes (tools, product shells, framework-only pages, etc.). |
| D4 | Where keywords and SEO data live | **Inside the Nota vault / Nota config directories only.** No cloud database. |
| D5 | Deployment / tenancy | **Local-first.** |
| D6 | Dual WP writers / separate wp-sync package | **Same as D1:** WP pull/push lives **inside Nota**. Leave `jsontoimg/scripts/wordpress-import.ts` **untouched**—it already works; day-to-day content work moves to Nota without modifying that script. |

## Site folder structure (rigid)

Each site under the Nota vault uses a fixed layout:

```text
sites/<site-id>/
  pages/          # Site / WordPress pages (URL-shaped folders)
  posts/          # Blog / WP posts (pillar + cluster via topic folders)
  static/         # Non-WP site routes only (never a second body for a WP URL)
  keywords/       # Keyword inventory, maps, weekly ranking audits
  assets/         # Images and media for this site
  notes/          # Optional freeform notes (not published pages)
  _archive/       # Retired / conflict / removed remote copies
  .nota/          # Machine state (inventory, media registry, sync hashes)
```

---

### `pages/` — durable site pages

Flat or nested folders that mirror public paths. Prefer `index.md` inside a folder when the URL is a directory-style path.

```text
pages/
  use-cases/
    index.md                 # /use-cases  (hub page)
    certificates.md          # /use-cases/certificates  (or certificates/index.md)
  alternatives/
    bannerbear-alternative.md
  legal/
    privacy.md
    terms.md
```

- Used for WordPress **pages** when CMS is connected, and for any non-blog page you manage in Nota.
- Each file has `url_path` (user-set or defaulted from path).
- Frontmatter typically: `source: wordpress|static`, `role: page`, `wp_id?`, `primary_keyword?`, `secondary_keywords?`.

---

### `posts/` — pillar vs cluster (folder convention)

Topic structure is **folder-based**:

```text
posts/
  <topic-slug>/
    index.md           # pillar post for this topic
    other-article.md   # cluster post
    another.md         # cluster post
```

- Creating a **folder** under `posts/` must create (or require) `index.md` — that file is the **pillar**.
- Every other `.md` file in the same folder is a **cluster** post belonging to that pillar.
- Frontmatter: `role: pillar` on `index.md`; `role: cluster` + `pillar: <topic-slug>` on siblings.
- Default `url_path`: pillar from folder; clusters from filename (e.g. `posts/credentials/certificate-generator-api-guide.md`).

---

### `static/` — framework-only / non-WP routes

Only content that is **not** owned by WordPress (D3). Same URL-folder habits as `pages/`, but never a duplicate body of a WP post/page.

```text
static/
  docs/
    api/
      index.md                 # /docs/api
    mcp/
      index.md
  tools/
    free-og-preview/
      index.md                 # /tools/free-og-preview
  product/
    image-generation-api/
      index.md
  about/
    index.md
```

- Framework-only sites put almost everything here.
- WP-CMS sites: only routes that are **not** pulled from WP (docs, tools, marketing shells you keep outside WP).
- Copy into Astro/Next is **manual** (D2).

---

### `keywords/` — inventory, assignment, weekly ranking audits

This is where you get the **whole picture**: which keywords exist, which article owns them, how they appear on-page, and how they ranked over time (via weekly MCP audits).

```text
keywords/
  index.yaml                 # master inventory (machine-readable SoT for keywords)
  keyword-page-map.md        # human-readable canonical targeting map (optional mirror)
  clusters.md                # topic / cluster planning notes (optional)
  audits/
    2026-11-18/
      ranking.yaml           # weekly snapshot: keyword → position, clicks, impressions, CTR, url
      summary.md             # human notes from that week’s audit
      improvements.md        # AI/human plan: what to change on which url_path
    2026-11-25/
      ranking.yaml
      summary.md
      improvements.md
```

**What you can see at a glance (joined views in Nota):**

| Question | Answered by |
|----------|-------------|
| How many keywords are we tracking? | Count of rows in `keywords/index.yaml` |
| Which are primary vs secondary? | `role` / `primary|secondary` on each row |
| Which article owns each keyword? | `target_url_path` (+ optional `wp_id`) → note in `pages/` / `posts/` / `static/` |
| Are two pages competing for the same primary? | Validator: one primary keyword → one URL |
| Is the keyword actually used in the article? | Note frontmatter `primary_keyword` / `secondary_keywords` + on-page checklist (title/H1/body) |
| What is ranking this week? | Latest `keywords/audits/<date>/ranking.yaml` from MCP (GSC and/or SERP) |
| Are we improving? | Compare audit folders week-over-week; `improvements.md` ties actions to URLs |

**Per-keyword row (conceptual fields in `index.yaml`):**  
keyword, cluster/topic, primary|secondary, intent, `target_url_path`, `wp_id?`, status (`planned|assigned|optimizing|monitoring`), last-audit position/clicks/impressions (copied or joined from latest audit).

**On each content note (frontmatter):**  
`primary_keyword`, `secondary_keywords[]` — must stay consistent with `index.yaml` (Nota validates both directions).

**Weekly ranking audit loop (MCP + AI):**

1. Run weekly audit (Nota command or agent using configured MCP, e.g. GSC): pull queries/pages/position/CTR for the site.
2. Write a dated folder under `keywords/audits/<YYYY-MM-DD>/` (`ranking.yaml` + short `summary.md`).
3. Join ranking rows to `index.yaml` by keyword and/or landing `url_path`.
4. Ask AI for an improvement plan → `improvements.md` (which URLs/keywords to strengthen, links to add, briefs to write)—**apply to notes only with explicit confirm**.
5. Next week: new audit folder; compare to previous week (gainers/losers/new opportunities).

History stays in the vault (D4)—no cloud DB required.

---

### `assets/` — media for this site

```text
assets/
  images/
    imported/                # pulled from WP media
    certificates/
    og/
  .placeholders/             # never push these to WP
```

- Featured images and body images referenced from notes.
- Sync/media registry metadata lives under `.nota/` (hashes ↔ WP media IDs); files live here.

---

### `notes/` — freeform (not published pages)

```text
notes/
  research/
    competitor-placid.md
  drafts/
    idea-mcp-landing.md
```

- Optional. Not part of the public URL inventory unless you promote a note into `pages/`, `posts/`, or `static/` (which assigns `url_path`).
- Good for research, meeting notes, AI scratchpads.

---

### `_archive/` — retired copies

```text
_archive/
  duplicate-static/          # old crawl duplicates
  deleted-on-wp/             # local notes whose remote WP id disappeared
  conflicts/                 # conflict copies from sync (if any)
```

- Prefer move-to-archive over silent delete when remediating D3 duplicates or remote deletions.

---

### `.nota/` — machine state (not editorial content)

```text
.nota/
  site.json                  # id, kind, public URL, wp API URL (no passwords)
  inventory.yaml             # known url_paths, sources, wp_ids
  media-registry.json        # sha256 ↔ vault path ↔ wp media id
  sync-state.json            # hashes / last pull-push metadata
  duplication-report.md      # optional generated report
```

- Credentials stay in Nota user settings / OS keychain, **not** in `.nota/` git if the vault is shared.
- Safe to regenerate inventory from disk + WP; do not hand-edit casually.

---

### URL / link rules

- Every managed content note has an explicit `url_path` (public link).
- When adding a **page or post**, user may set the link; **default** = folder + filename (e.g. `static/tools/free-og-preview/index.md` → `/tools/free-og-preview`).
- Changing folder location should offer to update `url_path` (or keep a manual override).
- Prefer **wikilinks** between notes for authoring; graph and WP push resolve them via `url_path`.

### Site types (chosen at “Add site”)

User must pick one:

| Type | Meaning |
|------|---------|
| **Framework-only** (Next.js / Astro / other) | No WordPress. Content mainly in `static/` (+ freeform `notes/`). Publish/copy to the framework repo is **manual**. Optional crawl of the live public URL into `static/` only for non-conflicting routes. |
| **WordPress CMS** | WP REST URL + username + Application Password. Pull/push into `posts/` and `pages/`. `static/` only for non-WP routes. |

A site may have **both** WP-managed URLs and framework-only static URLs; they stay in separate trees (D3).

## MCP and AI configuration

- Nota loads MCP server definitions similarly to Cursor’s `.mcp.json`.
- **Preferred location:** `<UserConfigDir>/nota/mcp.json` (same family as `keybindings.json`).
- **Optional per-vault override:** `<vault>/.nota/mcp.json`.
- Weekly ranking audits and AI improvement planning **use MCP tools** (e.g. GSC) and write results under `keywords/audits/`.
- Never send WP Application Passwords or MCP secrets to the language model.

## Changelog

- 2026-11-18 — Initial discovery defaults recorded.
- 2026-11-18 — Owner lock: merge wp-sync into Nota; MD-first manual framework copy; WP≠static duplication rule; local-only data; MCP config; site types; leave `wordpress-import.ts` alone.
- 2026-11-18 — Rename site tree `pillars/` → `posts/`. Pillar = folder `index.md`; other files in the folder = cluster posts.
- 2026-11-18 — Elaborate all site folders; keyword inventory + weekly MCP ranking audit → AI improvements loop under `keywords/audits/`.
