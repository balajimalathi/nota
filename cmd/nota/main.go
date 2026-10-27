package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"nota/internal/sync"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := serve(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	app := newApp()
	var open string
	if len(os.Args) > 1 && !isFlag(os.Args[1]) {
		open = os.Args[1]
	}
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "Nota",
			Width:   1200,
			Height:  800,
			Content: ui.View(app.view),
		})
		app.win.OnClose(func(*mygo.CloseEvent) { app.persist() })
		switch {
		case open != "":
			app.openVault(open)
		case app.settings.LastVault != "":
			if st, err := os.Stat(app.settings.LastVault); err == nil && st.IsDir() {
				app.openVault(app.settings.LastVault)
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8765", "listen address")
	dir := fs.String("vault", "", "vault directory")
	token := fs.String("token", "", "bearer token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" && fs.NArg() > 0 {
		*dir = fs.Arg(0)
	}
	if *dir == "" {
		return errors.New("usage: nota serve -vault <dir> [-addr host:port] [-token secret]")
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a directory", abs)
	}
	log.Printf("nota sync serving %s on http://%s", abs, *addr)
	return http.ListenAndServe(*addr, sync.Handler(abs, *token))
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
