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

// run opens the trade screen for a coin (HYPE by default), or with
// "list [filter]" prints every market.
func run(args []string) error {
	if len(args) > 0 && args[0] == "list" {
		ms, err := api.Markets()
		if err == nil {
			fmt.Println(ui.Table(ui.Filter(ms, strings.Join(args[1:], "")), -1))
		}
		return err
	}
	coin := "HYPE"
	if len(args) > 0 {
		coin = args[0]
	}
	m, err := api.Market(coin)
	if err != nil {
		return err
	}
	s := new(api.Stream)
	p := tea.NewProgram(app.New(m, s), tea.WithAltScreen())
	go s.Run(func(v any) { p.Send(v) })
	_, err = p.Run()
	return err
}
