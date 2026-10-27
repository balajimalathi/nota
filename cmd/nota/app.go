package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"

	"nota/internal/ai"
	"nota/internal/embed"
	"nota/internal/index"
	"nota/internal/keymap"
	"nota/internal/kit"
	"nota/internal/md"
	"nota/internal/note"
	"nota/internal/settings"
	"nota/internal/vault"
)

const (
	modeSource = iota
	modeLive
	modeRead
)

type noteTab struct {
	path    string
	state   *editor.State
	saveGen int
	lastVer uint64
}

type chatMsg struct {
	Role string
	Text string
	Done bool
}

type ghit struct {
	path string
	x, y float32
}

type sessionFile struct {
	Tabs      []string `json:"tabs"`
	Active    string   `json:"active"`
	Sidebar   float32  `json:"sidebar"`
	Editor    float32  `json:"editor"`
	ShowSide  bool     `json:"showSide"`
	ShowRight bool     `json:"showRight"`
	Mode      int      `json:"mode"`
	RightTab  int      `json:"rightTab"`
}

// App is the window's state. The view reads it every frame.
type App struct {
	settings settings.Settings
	bindings []keymap.Binding
	win      *mygo.Window

	vault  *vault.Vault
	index  *index.Index
	store  *embed.Store
	client *ai.Client

	tabs   []*noteTab
	active int

	roots    []string
	children map[string][]string
	dirs     map[string]bool
	outline  ui.OutlineState[string]
	fileRow  int
	lastTree string

	sidebarW  float32
	editorPct float32
	showSide  bool
	showRight bool
	rightTab  int
	mode      int
	tagFilter string

	quickOpen   bool
	quickQuery  string
	quickList   ui.ListState
	quickSel    int
	searchOpen  bool
	searchQuery string
	searchList  ui.ListState
	searchSel   int

	creating    bool
	renaming    bool
	settingsOn  bool
	syncOn      bool
	templatesOn bool
	deleting    bool
	modalText   string
	newDir      string
	status      string
	syncStatus  string

	aiMessages []chatMsg
	aiInput    string
	aiBusy     bool
	aiReach    string
	related    []embed.Hit

	watchCancel context.CancelFunc
	caretLine   atomic.Int32
	noteLang    *editor.Language
	editorBinds []editor.Binding
	mdOpts      markdown.Options
	pal         kit.Palette // this frame's colors, set first in view

	docCache   md.Document
	docVer     uint64
	docPath    string
	preview    string
	previewVer uint64
	previewFor string

	graphHits []ghit
	graphFG   ui.Color
	graphAC   ui.Color
	graphTX   ui.Color

	templates     []string
	tplSel        int
	pendingTabs   []string
	pendingActive string
}

type command struct {
	ID, Title string
}

func newApp() *App {
	s, err := settings.Load()
	if err != nil {
		s = settings.Defaults()
	}
	app := &App{
		settings:  s,
		bindings:  loadBindings(),
		index:     index.New(),
		store:     embed.New(),
		sidebarW:  260,
		editorPct: 72,
		showSide:  true,
		showRight: true,
		fileRow:   -1,
		aiReach:   "offline",
	}
	app.refreshClient()
	app.noteLang = &editor.Language{
		Name:       "Markdown",
		Extensions: []string{".md", ".markdown"},
		TabSize:    4,
		Tokenizer:  editor.TokenizerFunc(app.tokenize),
		AutoClose: []editor.BracketPair{
			{Open: "(", Close: ")"},
			{Open: "[", Close: "]"},
			{Open: "{", Close: "}"},
			{Open: "`", Close: "`"},
		},
	}
	app.editorBinds = app.editorKeys()
	return app
}

func (app *App) refreshClient() {
	s := app.settings
	app.client = ai.New(s.AIEndpoint, s.AIAPIKey, s.AIChatModel, s.AIEmbedModel)
}

func (app *App) editorKeys() []editor.Binding {
	var out []editor.Binding
	for _, b := range app.bindings {
		ch, ok := keymap.ParseChord(b.Key)
		if !ok {
			continue
		}
		key, ok := namedKey(ch.Key)
		if !ok {
			continue
		}
		out = append(out, editor.Binding{Mods: chordMods(ch), Key: key, Command: editor.NoCommand})
	}
	return out
}

func (app *App) openVault(path string) {
	v, err := vault.Open(path)
	if err != nil {
		app.status = err.Error()
		return
	}
	if app.vault != nil {
		app.persist()
	}
	if app.watchCancel != nil {
		app.watchCancel()
	}
	app.vault = v
	app.tabs = nil
	app.active = 0
	app.settings.Remember(v.Root())
	_ = settings.Save(app.settings)
	appDir, err := v.EnsureApp()
	if err != nil {
		app.status = err.Error()
		return
	}
	ix, err := index.Load(filepath.Join(appDir, "index.gob"))
	if err != nil || ix == nil {
		ix = index.New()
	}
	app.index = ix
	st, err := embed.Load(filepath.Join(appDir, "embeddings.bin"))
	if err != nil || st == nil {
		st = embed.New()
	}
	app.store = st
	app.loadSession(appDir)
	app.refreshTree()
	app.restoreTabs()
	app.mdOpts = markdown.Options{
		WrapCode:  true,
		OnLink:    app.openLink,
		LoadImage: markdown.FileImages(v.Root()),
		Theme: &markdown.Theme{
			FontSize: app.settings.FontSize,
			Mark:     ui.RGBA(250, 204, 21, 0.45),
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.watchCancel = cancel
	if ch, err := v.Watch(ctx); err == nil {
		go app.watchLoop(ch)
	} else {
		app.status = err.Error()
	}
	go app.catchUp(v, appDir)
	go app.pingAI()
	if app.win != nil {
		app.win.SetTitle("Nota — " + filepath.Base(v.Root()))
	}
}

func (app *App) catchUp(v *vault.Vault, appDir string) {
	files, err := v.Files()
	if err != nil {
		return
	}
	root := v.Root()
	have := map[string]bool{}
	app.index.Bulk(func() {
		for _, f := range files {
			if f.Dir || !strings.HasSuffix(strings.ToLower(f.Path), ".md") {
				continue
			}
			have[f.Path] = true
			if app.index.Fresh(f.Path, f.ModTime, f.Size) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
			if err != nil {
				continue
			}
			app.index.Upsert(f.Path, string(b), f.ModTime, f.Size)
		}
		for _, p := range app.index.Paths() {
			if !have[p] {
				app.index.Remove(p)
			}
		}
	})
	_ = app.index.Save(filepath.Join(appDir, "index.gob"))
	if app.win == nil {
		return
	}
	app.win.Update(func() {
		if app.vault == nil || app.vault.Root() != root {
			return
		}
		app.refreshTree()
	})
}

func (app *App) watchLoop(ch <-chan vault.Event) {
	var batch []vault.Event
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			batch = append(batch, ev)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(80 * time.Millisecond)
		case <-timer.C:
			events := batch
			batch = nil
			if app.win == nil {
				continue
			}
			app.win.Update(func() { app.applyEvents(events) })
		}
	}
}

func (app *App) applyEvents(evs []vault.Event) {
	if app.vault == nil {
		return
	}
	tree := false
	for _, ev := range evs {
		if ev.Op == "rescan" || ev.Path == "" {
			tree = true
			continue
		}
		if ev.Op == "remove" {
			if strings.HasSuffix(strings.ToLower(ev.Path), ".md") {
				app.index.Remove(ev.Path)
			}
			tree = true
			continue
		}
		if strings.HasSuffix(strings.ToLower(ev.Path), ".md") {
			b, err := app.vault.Read(ev.Path)
			if err != nil {
				continue
			}
			if t := app.tabByPath(ev.Path); t != nil {
				cur := t.state.Buffer.String()
				if cur == string(b) {
					if info, err := app.vault.Stat(ev.Path); err == nil {
						app.index.Upsert(ev.Path, cur, info.ModTime, info.Size)
					}
				} else if !t.state.Buffer.Modified() {
					t.state.Buffer.Reset(string(b))
					t.lastVer = t.state.Buffer.Version()
					if info, err := app.vault.Stat(ev.Path); err == nil {
						app.index.Upsert(ev.Path, string(b), info.ModTime, info.Size)
					}
				}
			} else if info, err := app.vault.Stat(ev.Path); err == nil {
				app.index.Upsert(ev.Path, string(b), info.ModTime, info.Size)
			}
		}
		tree = true
	}
	if tree {
		app.refreshTree()
	}
}

func (app *App) refreshTree() {
	if app.vault == nil {
		return
	}
	files, err := app.vault.Files()
	if err != nil {
		app.status = err.Error()
		return
	}
	kids := map[string][]string{}
	dirs := map[string]bool{}
	var roots []string
	for _, f := range files {
		if f.Dir {
			dirs[f.Path] = true
		}
		parent := filepath.ToSlash(filepath.Dir(f.Path))
		if parent == "." {
			roots = append(roots, f.Path)
			continue
		}
		kids[parent] = append(kids[parent], f.Path)
	}
	sortTree(roots, dirs)
	for _, list := range kids {
		sortTree(list, dirs)
	}
	app.roots = roots
	app.children = kids
	app.dirs = dirs
}

func sortTree(list []string, dirs map[string]bool) {
	for i := 1; i < len(list); i++ {
		j := i
		for j > 0 && treeLess(list[j], list[j-1], dirs) {
			list[j], list[j-1] = list[j-1], list[j]
			j--
		}
	}
}

func treeLess(a, b string, dirs map[string]bool) bool {
	if dirs[a] != dirs[b] {
		return dirs[a]
	}
	return a < b
}

func (app *App) childPaths(p string) []string {
	c := app.children[p]
	if len(c) == 0 {
		return nil
	}
	return c
}

func (app *App) commands() []command {
	return []command{
		{"quickOpen", "Quick open"},
		{"search", "Search in notes"},
		{"newNote", "New note"},
		{"dailyNote", "Open today's note"},
		{"save", "Save"},
		{"closeTab", "Close tab"},
		{"toggleSidebar", "Toggle file tree"},
		{"toggleRight", "Toggle side pane"},
		{"cycleMode", "Cycle source, live, and reading"},
		{"rename", "Rename note"},
		{"delete", "Delete note"},
		{"insertTemplate", "Insert template"},
		{"graph", "Show graph"},
		{"summarize", "Summarize note"},
		{"rewrite", "Rewrite selection"},
		{"continue", "Continue writing"},
		{"title", "Suggest title and tags"},
		{"ask", "Ask the vault"},
		{"indexVault", "Index vault for Ask"},
		{"sync", "Sync"},
		{"settings", "Settings"},
		{"openVault", "Open vault"},
	}
}

func (app *App) run(id string) {
	switch id {
	case "quickOpen", "commandPalette":
		app.quickOpen, app.quickQuery, app.quickSel = true, "", 0
	case "search":
		app.searchOpen = true
		app.searchSel = 0
	case "newNote":
		app.newDir = ""
		app.modalText = ""
		app.creating = true
	case "dailyNote":
		app.daily()
	case "save":
		app.saveTab(app.current())
	case "closeTab":
		app.closeTab(app.active)
	case "toggleSidebar":
		app.showSide = !app.showSide
	case "toggleRight":
		app.showRight = !app.showRight
	case "cycleMode":
		app.mode = (app.mode + 1) % 3
	case "rename":
		if t := app.current(); t != nil {
			app.modalText = note.TitleFromName(t.path)
			app.renaming = true
		}
	case "delete":
		if app.current() != nil {
			app.deleting = true
		}
	case "insertTemplate":
		app.templates = app.templateFiles()
		app.templatesOn = true
	case "graph":
		app.showRight = true
		app.rightTab = 3
	case "summarize", "rewrite", "continue", "title":
		app.showRight = true
		app.rightTab = 4
		app.runAI(id)
	case "ask":
		app.showRight = true
		app.rightTab = 4
		app.askVault()
	case "indexVault":
		app.indexVault()
	case "sync":
		app.syncOn = true
		app.refreshSync()
	case "settings":
		app.settingsOn = true
	case "openVault":
		app.pickVault()
	}
}

func (app *App) shortcuts(c *ui.Context) {
	if app.quickOpen || app.searchOpen || app.creating || app.renaming || app.settingsOn || app.syncOn || app.templatesOn || app.deleting {
		return
	}
	for _, b := range app.bindings {
		ch, ok := keymap.ParseChord(b.Key)
		if !ok {
			continue
		}
		key, ok := namedKey(ch.Key)
		if !ok {
			continue
		}
		if c.Shortcut(chordMods(ch), key) {
			app.run(b.Command)
			return
		}
	}
}

func (app *App) pickVault() {
	win := app.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win, Title: "Open vault", Directory: true,
		})
		if err != nil || len(paths) == 0 || win == nil {
			return
		}
		win.Update(func() { app.openVault(paths[0]) })
	}()
}

func (app *App) current() *noteTab {
	if app.active < 0 || app.active >= len(app.tabs) {
		return nil
	}
	return app.tabs[app.active]
}

func (app *App) tabByPath(p string) *noteTab {
	for _, t := range app.tabs {
		if t.path == p {
			return t
		}
	}
	return nil
}

func (app *App) openNote(rel string) {
	if app.vault == nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if strings.Contains(rel, "..") {
		return
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		_ = mygo.Shell.OpenPath(filepath.Join(app.vault.Root(), filepath.FromSlash(rel)))
		return
	}
	for i, t := range app.tabs {
		if t.path == rel {
			app.active = i
			return
		}
	}
	b, err := app.vault.Read(rel)
	if err != nil {
		app.status = err.Error()
		return
	}
	st := app.newState(string(b))
	st.Label = rel
	app.tabs = append(app.tabs, &noteTab{path: rel, state: st, lastVer: st.Buffer.Version()})
	app.active = len(app.tabs) - 1
}

func (app *App) newState(text string) *editor.State {
	buf := editor.NewBuffer(text)
	buf.SetLanguage(app.noteLang)
	st := &editor.State{
		Buffer:       buf,
		SoftWrap:     app.settings.SoftWrap,
		Bindings:     app.editorBinds,
		OnCompletion: app.complete,
		OnDefinition: app.gotoDef,
		OnHover:      app.hover,
		Placeholder:  "Start writing…",
		LineHeight:   1.45,
	}
	if app.settings.FontSize > 0 {
		st.Font.Size = app.settings.FontSize
	}
	return st
}

func (app *App) saveTab(t *noteTab) {
	if t == nil || app.vault == nil || t.state.Buffer == nil {
		return
	}
	if !t.state.Buffer.Modified() {
		return
	}
	var buf strings.Builder
	if _, err := t.state.Buffer.WriteTo(&buf); err != nil {
		app.status = err.Error()
		return
	}
	data := buf.String()
	if err := app.vault.Write(t.path, []byte(data)); err != nil {
		app.status = err.Error()
		return
	}
	t.state.Buffer.MarkSaved()
	info, err := app.vault.Stat(t.path)
	if err == nil {
		app.index.Upsert(t.path, data, info.ModTime, info.Size)
	}
	app.status = "Saved " + t.path
}

func (app *App) autosave(t *noteTab) {
	if t == nil || !t.state.Buffer.Modified() {
		return
	}
	ver := t.state.Buffer.Version()
	if ver == t.lastVer {
		return
	}
	t.lastVer = ver
	t.saveGen++
	gen := t.saveGen
	path := t.path
	go func() {
		time.Sleep(450 * time.Millisecond)
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			cur := app.tabByPath(path)
			if cur == nil || cur.saveGen != gen {
				return
			}
			app.saveTab(cur)
		})
	}()
}

func (app *App) closeTab(i int) {
	if i < 0 || i >= len(app.tabs) {
		return
	}
	app.saveTab(app.tabs[i])
	app.tabs = append(app.tabs[:i], app.tabs[i+1:]...)
	if app.active >= len(app.tabs) {
		app.active = len(app.tabs) - 1
	}
	if app.active < 0 {
		app.active = 0
	}
}

func (app *App) createNamed(name string) {
	if app.vault == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	rel := notePath(name)
	if app.newDir != "" {
		rel = app.newDir + "/" + rel
	}
	if _, err := app.vault.Stat(rel); err == nil {
		app.openNote(rel)
		return
	}
	body := "# " + note.TitleFromName(rel) + "\n\n"
	if err := app.vault.Write(rel, []byte(body)); err != nil {
		app.status = err.Error()
		return
	}
	info, _ := app.vault.Stat(rel)
	app.index.Upsert(rel, body, info.ModTime, info.Size)
	app.refreshTree()
	app.openNote(rel)
}

func (app *App) daily() {
	if app.vault == nil {
		return
	}
	rel := note.DailyRel(app.settings.DailyFormat, time.Now())
	if _, err := app.vault.Stat(rel); err != nil {
		body := "# " + note.TitleFromName(rel) + "\n\n"
		dir := app.settings.TemplatesDir
		if dir == "" {
			dir = "templates"
		}
		if b, err := app.vault.Read(dir + "/daily.md"); err == nil {
			body = note.ApplyTemplate(string(b), note.TitleFromName(rel), time.Now())
		}
		if err := app.vault.Write(rel, []byte(body)); err != nil {
			app.status = err.Error()
			return
		}
		info, _ := app.vault.Stat(rel)
		app.index.Upsert(rel, body, info.ModTime, info.Size)
		app.refreshTree()
	}
	app.openNote(rel)
}

func (app *App) doRename(name string) {
	t := app.current()
	if t == nil || app.vault == nil {
		return
	}
	app.saveTab(t)
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	next := notePath(name)
	dir := filepath.ToSlash(filepath.Dir(t.path))
	if dir != "." && dir != "" {
		next = dir + "/" + filepath.Base(next)
	}
	if next == t.path {
		return
	}
	type edit struct{ path, body string }
	var edits []edit
	for _, p := range app.index.Paths() {
		b, err := app.vault.Read(p)
		if err != nil {
			continue
		}
		out, n := app.index.Rewrite(string(b), t.path, next)
		if n > 0 {
			dest := p
			if p == t.path {
				dest = next
			}
			edits = append(edits, edit{dest, out})
		}
	}
	if err := app.vault.Rename(t.path, next); err != nil {
		app.status = err.Error()
		return
	}
	old := t.path
	t.path = next
	t.state.Label = next
	for _, e := range edits {
		if err := app.vault.Write(e.path, []byte(e.body)); err != nil {
			app.status = err.Error()
			continue
		}
		if e.path == next {
			t.state.Buffer.Reset(e.body)
			t.state.Buffer.MarkSaved()
		}
		if info, err := app.vault.Stat(e.path); err == nil {
			app.index.Upsert(e.path, e.body, info.ModTime, info.Size)
		}
	}
	app.index.Remove(old)
	if info, err := app.vault.Stat(next); err == nil {
		app.index.Upsert(next, t.state.Buffer.String(), info.ModTime, info.Size)
	}
	app.refreshTree()
	app.status = "Renamed to " + next
}

func (app *App) doDelete() {
	t := app.current()
	if t == nil || app.vault == nil {
		return
	}
	path := t.path
	app.closeTab(app.active)
	if err := app.vault.Remove(path); err != nil {
		app.status = err.Error()
		return
	}
	app.index.Remove(path)
	app.refreshTree()
	app.status = "Deleted " + path
}

func (app *App) templateFiles() []string {
	if app.vault == nil {
		return nil
	}
	dir := app.settings.TemplatesDir
	if dir == "" {
		dir = "templates"
	}
	abs := filepath.Join(app.vault.Root(), filepath.FromSlash(dir))
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			out = append(out, dir+"/"+e.Name())
		}
	}
	return out
}

func (app *App) insertTemplate(rel string) {
	t := app.current()
	if t == nil || app.vault == nil {
		return
	}
	b, err := app.vault.Read(rel)
	if err != nil {
		app.status = err.Error()
		return
	}
	body := note.ApplyTemplate(string(b), app.index.Title(t.path), time.Now())
	t.state.Insert(body)
}

func (app *App) openLink(raw string) {
	switch {
	case strings.HasPrefix(raw, "nota://open/"):
		rest := strings.TrimPrefix(raw, "nota://open/")
		name, frag, _ := strings.Cut(rest, "#")
		name, err := url.PathUnescape(name)
		if err != nil {
			return
		}
		if p, ok := app.index.Resolve(name); ok {
			app.openNote(p)
			if frag != "" {
				app.revealHeading(frag)
			}
			return
		}
		app.createNamed(name)
	case strings.HasPrefix(raw, "nota://tag/"):
		tag, err := url.PathUnescape(strings.TrimPrefix(raw, "nota://tag/"))
		if err != nil {
			return
		}
		app.tagFilter = tag
		app.showRight = true
		app.rightTab = 2
	default:
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "mailto:") {
			_ = mygo.Shell.OpenExternal(raw)
		}
	}
}

func (app *App) revealHeading(frag string) {
	t := app.current()
	if t == nil {
		return
	}
	want := strings.ToLower(strings.TrimPrefix(frag, "#"))
	doc := md.Parse(t.state.Buffer.String())
	for _, h := range doc.Headings {
		if strings.EqualFold(h.Text, want) || slug(h.Text) == want {
			t.state.GoTo(editor.Pos{Line: h.Line, Col: 0})
			return
		}
	}
}

func (app *App) loadSession(appDir string) {
	b, err := os.ReadFile(filepath.Join(appDir, "session.json"))
	if err != nil {
		return
	}
	var s sessionFile
	if json.Unmarshal(b, &s) != nil {
		return
	}
	if s.Sidebar > 80 {
		app.sidebarW = s.Sidebar
	}
	if s.Editor > 20 && s.Editor < 90 {
		app.editorPct = s.Editor
	}
	app.showSide = s.ShowSide
	app.showRight = s.ShowRight
	app.mode = s.Mode
	app.rightTab = s.RightTab
	app.pendingTabs = s.Tabs
	app.pendingActive = s.Active
}

func (app *App) restoreTabs() {
	for _, p := range app.pendingTabs {
		if _, err := app.vault.Stat(p); err == nil {
			app.openNote(p)
		}
	}
	for i, t := range app.tabs {
		if t.path == app.pendingActive {
			app.active = i
		}
	}
	app.pendingTabs = nil
}

func (app *App) persist() {
	if app.vault == nil {
		_ = settings.Save(app.settings)
		return
	}
	for _, t := range app.tabs {
		app.saveTab(t)
	}
	s := sessionFile{
		Active:    "",
		Sidebar:   app.sidebarW,
		Editor:    app.editorPct,
		ShowSide:  app.showSide,
		ShowRight: app.showRight,
		Mode:      app.mode,
		RightTab:  app.rightTab,
	}
	for _, t := range app.tabs {
		s.Tabs = append(s.Tabs, t.path)
	}
	if cur := app.current(); cur != nil {
		s.Active = cur.path
	}
	appDir, err := app.vault.EnsureApp()
	if err == nil {
		if b, err := json.MarshalIndent(s, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(appDir, "session.json"), append(b, '\n'), 0o644)
		}
		_ = app.index.Save(filepath.Join(appDir, "index.gob"))
		_ = app.store.Save(filepath.Join(appDir, "embeddings.bin"))
	}
	_ = settings.Save(app.settings)
}

func (app *App) pingAI() {
	err := app.client.Reachable(context.Background())
	if app.win == nil {
		return
	}
	app.win.Update(func() {
		if err != nil {
			app.aiReach = "offline"
			return
		}
		app.aiReach = "ready"
	})
}

func loadBindings() []keymap.Binding {
	dir, err := settings.Dir()
	if err != nil {
		return keymap.Default()
	}
	p := filepath.Join(dir, "keybindings.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			if raw, err := json.MarshalIndent(keymap.Default(), "", "  "); err == nil {
				_ = os.WriteFile(p, append(raw, '\n'), 0o644)
			}
		}
		return keymap.Default()
	}
	got, err := keymap.Parse(b)
	if err != nil || len(got) == 0 {
		return keymap.Default()
	}
	return got
}

func notePath(name string) string {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	name = filepath.ToSlash(name)
	if strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return name
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
