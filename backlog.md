# Nota SEO CMS — Implementation Backlog

> **Source of truth for product rules:** [seo-cms-decisions.md](docs/superpowers/specs/2026-11-18-seo-cms-decisions.md)  
> **Evidence:** [repository-discovery-architecture-audit.md](docs/superpowers/specs/2026-11-18-repository-discovery-architecture-audit.md)  
> **For agentic workers:** Implement against the decisions doc. Prefer small PRs. WordPress DELETE stays disabled. Do **not** modify `jsontoimg/scripts/wordpress-import.ts`. Obsidian and a separate wp-sync CLI are not part of the target workflow.

**Goal:** Local-first SEO content workspace in Nota: rigid per-site Markdown folders, keyword inventory + weekly MCP ranking audits + AI improvement plans, WordPress pull/push when the site uses WP as CMS—without duplicating WP content into `static/`, and without auto-writing Astro/Next repos.

---

## Locked architecture (from decisions)

1. **Merge wp-sync into Nota** (Go-native preferred). No separate CLI; no Obsidian plugin. MD stays Obsidian-compatible.
2. **Nota Markdown is the working SoT.** Copy to Astro/Next is **manual**.
3. **Rigid site tree:**

```text
sites/<site-id>/
  pages/ posts/ static/ keywords/ assets/ notes/ _archive/ .nota/
```

4. **`posts/<topic>/index.md`** = pillar; sibling `.md` = clusters.
5. **D3:** WP-owned URLs live only under `pages/` / `posts/`; `static/` never holds a second body for the same content.
6. **Every managed note** has `url_path` (user-set or defaulted from folder+filename). Prefer **wikilinks** in bodies; graph/push resolve via `url_path`.
7. **Keywords + audits** live under `keywords/` (vault-local). No cloud DB.
8. **MCP:** `<UserConfigDir>/nota/mcp.json` (+ optional vault `.nota/mcp.json`). Weekly audits write `keywords/audits/<date>/`.
9. **Site kinds at add-site:** `framework_only` | `wordpress_cms`.

| ID | Locked resolution |
|----|-------------------|
| D1 | Merge wp-sync into Nota; Obsidian retired as editor |
| D2 | MD in Nota; framework copy manual |
| D3 | WP ≠ static duplicate bodies |
| D4 | Vault / Nota config only |
| D5 | Local-first |
| D6 | WP sync inside Nota; leave `wordpress-import.ts` alone |

---

## Epic 0 — Docs aligned with decisions

- [x] Discovery audit published.
- [x] Decision log locked (D1–D6 + folder model + keywords/audits + MCP).
- [x] README related-projects section.
- [x] Backlog rewritten to match decisions (this file).

**Done when:** Backlog and decisions do not contradict (no `pillars/` tree, no CLI-spawn MVP, no Astro-as-docs-SoT).

---

## Epic 1 — Site model and rigid folders

**Outcome:** Add site, create the full tree, create pages/posts with correct roles and default `url_path`.

### 1.1 Registry and site types

- [x] Persist per-site `.nota/site.json`: `id`, `name`, `publicSiteUrl`, `kind` ∈ {`framework_only`, `wordpress_cms`}, optional `framework` (`astro`|`next`|`other`), optional `wpApiUrl` (no passwords in file).
- [ ] WP credentials in Nota user settings / OS keychain only. (settings storage in progress)
- [ ] Multi-site: vault may contain `sites/<id>/…`; switch active site in UI. (core site switch scaffolded)

### 1.2 Create rigid directories on site add

- [x] Ensure: `pages/`, `posts/`, `static/`, `keywords/` (incl. empty `audits/`), `assets/images/`, `notes/`, `_archive/`, `.nota/`.
- [x] Seed empty `keywords/index.yaml` and `.nota/inventory.yaml` as needed.

### 1.3 `pages/`

- [x] New page: nested folders allowed; prefer `index.md` for directory URLs. (core `CreatePage`)
- [x] Frontmatter: `title`, `url_path`, `source` (`wordpress`|`static`), `role: page`, optional `wp_id`, `primary_keyword`, `secondary_keywords`, `status`.
- [x] Default `url_path` from path (e.g. `pages/use-cases/certificates.md` → `/use-cases/certificates`).

### 1.4 `posts/` pillar / cluster

- [x] “New topic folder”: create `posts/<topic-slug>/index.md` with `role: pillar`.
- [x] “New cluster post”: sibling `.md` with `role: cluster`, `pillar: <topic-slug>`.
- [x] Default `url_path`: pillar from folder; cluster from filename. (tested)
- [ ] UI/file-tree treats `index.md` as pillar; forbids topic folder without `index.md` (create or prompt). (core `ValidatePostFolder`)

### 1.5 `static/`, `notes/`, `assets/`

- [x] `static/`: same URL defaults as pages; only for non-WP routes (enforce when `kind=wordpress_cms`). (URL path in `DefaultURLPath`)
- [ ] `notes/`: freeform; **no** `url_path` until promoted into `pages/` / `posts/` / `static/`. (intentional in layout)
- [ ] `assets/`: image paths for notes; placeholders never push to WP. (dir exists, no push logic yet)

### 1.6 Migration

- [ ] Open existing `obsidian/sites/jsontoimg` as a site. (migration scaffold not yet done)
- [ ] Map legacy `pillars/` → `posts/` (topic folders + promote category index to `index.md` where needed). (not started)
- [ ] Map `.wp-sync/` → `.nota/` (dual-read during transition OK). (not started)

**Done when:** Framework-only and WP sites can be added; new page/post get default links; post folders enforce pillar `index.md`.

---

## Epic 2 — WP vs static separation (D3)

**Outcome:** No second editable body for WP-owned URLs.

- [ ] Build `.nota/inventory.yaml` from disk (+ WP when connected): path, `source`, `url_path`, `wp_id`, role.
- [ ] Generate `.nota/duplication-report.md`: static overlapping WP by `url_path` / `wp_id` / similarity.
- [ ] Remediate: archive duplicates under `_archive/duplicate-static/` (or `conflicts/`); keep WP note in `pages/` or `posts/` only.
- [ ] Crawl/bootstrap: URLs matched to WP → WP tree only; never write full static bodies for claimed WP URLs.
- [ ] Prefer no static stubs; inventory is enough.

**Done when:** Active site has zero dual bodies for WP-owned URLs; report clean or waived.

---

## Epic 3 — WordPress sync inside Nota (D1/D6)

**Outcome:** Bootstrap / Pull / Push / Validate from Nota; conflict-safe; no Obsidian/CLI required.

### 3.1 Port into Nota

- [ ] Port/rewrite from `obsidian-wp-sync/packages/core`: REST (GET/POST only), HTML↔MD, media registry → `assets/` + `.nota/media-registry.json`, pull into `posts/` & `pages/`, push, validate.
- [ ] Packages e.g. `internal/wp`, `internal/site`.
- [ ] UI/commands for active WP site; layout posts as `posts/<topic>/index.md` + clusters (map WP categories/topics into folders).

### 3.2 Conflict safety

- [ ] Persist hashes + remote modified in frontmatter / `.nota/sync-state.json`.
- [ ] Pull: no silent overwrite of dirty local; conflict copies under `_archive/conflicts/` or Nota conflict naming.
- [ ] Push: refuse when remote diverged unless forced; skip when unchanged.
- [ ] Tests: dirty-local, dirty-remote, clean skip.

### 3.3 Metadata

- [ ] Tags round-trip; categories preserved; featured images via registry; **no WP DELETE**.
- [ ] **Do not change** `jsontoimg/scripts/wordpress-import.ts`.

**Done when:** Nota-only pull→edit→push on staging WP; tests green.

---

## Epic 4 — Keyword inventory (whole picture)

**Outcome:** Track how many keywords, ownership, collisions, and assignment—all under `keywords/`.

### 4.1 Files (per decisions)

```text
keywords/
  index.yaml
  keyword-page-map.md      # optional human mirror
  clusters.md              # optional planning
  audits/                  # Epic 6 writes here
```

- [ ] `index.yaml` rows: keyword, cluster/topic, primary|secondary, intent, `target_url_path`, `wp_id?`, status (`planned|assigned|optimizing|monitoring`), optional last-audit metrics join.
- [ ] Note frontmatter: `primary_keyword`, `secondary_keywords[]` — bidirectional validate vs `index.yaml`.
- [ ] Validator: one primary → one URL; flag orphan keywords and notes missing primaries.

### 4.2 Import + UX

- [ ] Import legacy CSV / page maps into `index.yaml`.
- [ ] Views: total count; by status/cluster; primaries with/without ranking; “keywords for note” / “notes for keyword”.

**Done when:** Counts and ownership visible offline; collision/missing reports work.

---

## Epic 5 — On-page SEO and internal links

**Outcome:** Keyword usage on-page + link graph for connected content.

- [ ] Frontmatter: `seo_title`, `meta_description` (plus existing `url_path`).
- [ ] Checklist: primary in title/H1/body; secondary coverage.
- [ ] Authoring: prefer wikilinks; extract wikilinks + markdown + absolute site URLs.
- [ ] Graph: edges `(from_url_path → to_url_path)`, type `existing` | `suggested`; site-filtered neighborhood (reuse Nota graph).
- [ ] Push: rewrite wikilinks → public `url_path`s for WP HTML.
- [ ] Spike only: WP SEO plugin meta via REST; keep local fields if unstable.

**Done when:** Sample of 10 URLs matches manual on-page + link audit.

---

## Epic 6 — MCP + weekly ranking audit

**Outcome:** Configure MCP; run weekly audit; store history; draft improvements.

### 6.1 MCP config

- [ ] Load `<UserConfigDir>/nota/mcp.json` (Cursor-compatible).
- [ ] Optional merge `<vault-or-site>/.nota/mcp.json` (site-level under `sites/<id>/.nota/` preferred when multi-site).
- [ ] Document format / settings UI; never send MCP secrets or WP passwords to the LLM.

### 6.2 Weekly loop (as in decisions)

```text
keywords/audits/<YYYY-MM-DD>/
  ranking.yaml
  summary.md
  improvements.md
```

- [ ] Command/agent: “Run weekly keyword audit” → MCP (GSC primary; optional SERP for gaps like volume/KD) → write audit folder.
- [ ] Join ranking rows to `index.yaml` by keyword and/or landing `url_path`.
- [ ] AI drafts `improvements.md` (URL + keyword actions, links, briefs); **apply to notes only on confirm**.
- [ ] Compare prior audit folder: gainers / losers / new opportunities.

**Done when:** One dated audit joins inventory; week-over-week compare works with two folders; improvement draft generated.

---

## Epic 7 — AI on connected content

**Outcome:** Improve articles using inventory, audits, links, and MCP—not only the open buffer.

- [ ] Actions: content brief; on-page recommendations; internal-link suggestions; execute items from latest `improvements.md`.
- [ ] Tool loop: model + configured MCP tools where relevant.
- [ ] Explicit apply-to-note; reuse `internal/ai` OpenAI-compatible client.

**Done when:** “Improve this article” uses keyword + latest audit context on a real site vault.

---

## Out of scope

- Auto-writing Astro/Next repos from Nota  
- Cloud multi-user auth / hosted DB  
- WordPress DELETE from Nota  
- Editing `jsontoimg/scripts/wordpress-import.ts`  
- Requiring Obsidian or a separate wp-sync CLI  

---

## Sequence

```text
Epic 0 → Epic 1 → Epic 2 → Epic 3
                ↘ Epic 4 → Epic 5 → Epic 6 → Epic 7
```

Epic 3 may parallel Epic 1 after path/layout rules are implemented. Do not crawl WP URLs into `static/` before Epic 2 rules exist. Epic 6 needs Epic 4 inventory; Epic 7 needs 4–6 for full value (thin AI can ship earlier on open note only).

---

## Verification checklist

- [x] `sites/<id>/{pages,posts,static,keywords,assets,notes,_archive,.nota}/` created on add-site. (core `EnsureLayout`)
- [ ] `posts/<topic>/index.md` is pillar; siblings are clusters.
- [ ] Default `url_path` from folder; override allowed at create.
- [ ] Framework-only vs WordPress CMS chosen explicitly.
- [ ] No active dual MD bodies for WP-owned URLs; duplicates in `_archive/`.
- [ ] Pull/push from Nota only (no Obsidian/CLI).
- [ ] `keywords/index.yaml` counts + ownership + collision checks.
- [ ] Weekly `keywords/audits/<date>/{ranking,summary,improvements}` via MCP.
- [ ] Wikilinks resolve in graph and on WP push via `url_path`.
- [ ] `mcp.json` loaded from Nota config.
- [ ] `wordpress-import.ts` untouched.
- [ ] Decisions doc remains the product rulebook; this backlog tracks work only.
