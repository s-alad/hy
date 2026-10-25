package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/app"
	"hy/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hy:", err)
		os.Exit(1)
	}
}

// run opens a market's trade screen (HYPE by default), or the markets list: hy list [perps|spot|hip3|dex] [name].
func run(args []string) error {
	ms, err := api.Markets()
	if err != nil {
		return err
	}
	var m api.Asset // none on the markets list
	var q string    // the list's filter
	switch {
	case len(args) > 0 && args[0] == "list" && !tty():
		_, err = fmt.Println(ui.Table(ui.Filter(ms, strings.Join(args[1:], " ")), -1, ""))
		return err
	case len(args) > 0 && args[0] == "list":
		q = strings.Join(args[1:], " ")
	case len(args) > 0:
		m, err = api.Find(ms, args[0])
	default:
		m, err = api.Find(ms, "HYPE")
	}
	if err != nil {
		return err
	}
	s := new(api.Stream)
	p := tea.NewProgram(app.New(m, ms, q, s), tea.WithAltScreen())
	go s.Run(func(v any) { p.Send(v) })
	_, err = p.Run()
	return err
}

// tty is whether stdout is a terminal; piped output is plain text.
func tty() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
