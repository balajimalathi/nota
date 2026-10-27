package main

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/settings"
	"nota/internal/sync"
)

func (app *App) syncModal(c *ui.Context) {
	ui.Modal(c, &app.syncOn, func() {
		ui.Text(c, "Sync").Bold()
		ui.Text(c, "Notes stay files. Git, a folder, or nota serve.").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(6).Children(func() {
			for _, mode := range []string{"off", "git", "folder", "http"} {
				m := mode
				label := m
				if m == app.settings.SyncMode {
					label = "● " + m
				}
				if ui.Button(c, label).Clicked() {
					app.settings.SyncMode = m
					_ = settings.Save(app.settings)
					app.refreshSync()
				}
			}
		})
		ui.Text(c, "Remote folder, or http://host:8765").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.SyncRemote).Width(360)
		ui.Text(c, "Token").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.SyncToken).Width(360)
		if app.syncStatus != "" {
			ui.Text(c, app.syncStatus)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "Status").Clicked() {
				app.refreshSync()
			}
			if ui.Button(c, "Pull").Clicked() {
				app.syncDo(false)
			}
			if ui.PrimaryButton(c, "Push").Clicked() {
				app.syncDo(true)
			}
			if app.settings.SyncMode == "git" && ui.Button(c, "git init").Clicked() {
				g := &sync.Git{Root: app.vault.Root(), Device: app.settings.Device}
				if err := g.Init(context.Background()); err != nil {
					app.syncStatus = err.Error()
				} else {
					app.syncStatus = "Repository created"
				}
			}
		})
		if ui.Button(c, "Close").Clicked() {
			_ = settings.Save(app.settings)
			app.syncOn = false
		}
	})
}

func (app *App) refreshSync() {
	if app.vault == nil {
		return
	}
	b, err := app.backend()
	if err != nil {
		app.syncStatus = err.Error()
		return
	}
	go func() {
		st, err := b.Status(context.Background())
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			if err != nil {
				app.syncStatus = err.Error()
				return
			}
			app.syncStatus = formatStatus(st)
		})
	}()
}

func (app *App) syncDo(push bool) {
	if app.vault == nil {
		return
	}
	_ = settings.Save(app.settings)
	b, err := app.backend()
	if err != nil {
		app.syncStatus = err.Error()
		return
	}
	app.syncStatus = "Working…"
	go func() {
		var err error
		if push {
			err = b.Push(context.Background())
		} else {
			err = b.Pull(context.Background())
		}
		if app.win == nil {
			return
		}
		app.win.Update(func() {
			if err != nil {
				app.syncStatus = err.Error()
			} else if push {
				app.syncStatus = "Pushed"
			} else {
				app.syncStatus = "Pulled"
			}
			app.reloadFromDisk()
			app.refreshSync()
		})
	}()
}

func (app *App) backend() (sync.Backend, error) {
	if app.vault == nil {
		return nil, errEmpty()
	}
	root := app.vault.Root()
	switch app.settings.SyncMode {
	case "git":
		return &sync.Git{Root: root, Device: app.settings.Device}, nil
	case "folder":
		if strings.TrimSpace(app.settings.SyncRemote) == "" {
			return nil, errString("Set a remote folder")
		}
		return &sync.Folder{Local: root, Remote: app.settings.SyncRemote, Device: app.settings.Device}, nil
	case "http":
		if strings.TrimSpace(app.settings.SyncRemote) == "" {
			return nil, errString("Set a server URL")
		}
		return &sync.HTTP{
			Local: root, BaseURL: app.settings.SyncRemote,
			Token: app.settings.SyncToken, Device: app.settings.Device,
		}, nil
	default:
		return nil, errString("Sync is off")
	}
}

func (app *App) reloadFromDisk() {
	if app.vault == nil {
		return
	}
	app.refreshTree()
	for _, t := range app.tabs {
		if t.state.Buffer.Modified() {
			continue
		}
		b, err := app.vault.Read(t.path)
		if err != nil {
			continue
		}
		if t.state.Buffer.String() != string(b) {
			t.state.Buffer.Reset(string(b))
			t.lastVer = t.state.Buffer.Version()
		}
	}
	v := app.vault
	dir, err := v.EnsureApp()
	if err != nil {
		return
	}
	go app.catchUp(v, dir)
}

func formatStatus(st sync.Status) string {
	var bits []string
	bits = append(bits, st.Mode)
	if st.Detail != "" {
		bits = append(bits, st.Detail)
	}
	if st.Ahead > 0 {
		bits = append(bits, itoa(st.Ahead)+" ahead")
	}
	if st.Behind > 0 {
		bits = append(bits, itoa(st.Behind)+" behind")
	}
	if len(st.Dirty) > 0 {
		bits = append(bits, itoa(len(st.Dirty))+" changed")
	}
	if len(st.Conflicts) > 0 {
		bits = append(bits, "conflicts: "+strings.Join(st.Conflicts, ", "))
	}
	if len(bits) == 1 && st.Detail == "" {
		return st.Mode + " · nothing waiting"
	}
	return strings.Join(bits, " · ")
}

type stringErr string

func (e stringErr) Error() string { return string(e) }
func errString(s string) error    { return stringErr(s) }
