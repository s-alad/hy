package main

import (
	"fmt"
	"os"
	"os/exec"
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

// run opens a market's trade screen (HYPE by default), or lists markets: hy list [perps|spot|hip3|dex] [name].
func run(args []string) error {
	if len(args) > 0 && args[0] == "list" {
		ms, err := api.Markets()
		if err != nil {
			return err
		}
		return page(ui.Table(ui.Filter(ms, strings.Join(args[1:], " ")), -1))
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

// page shows text like bat: through less in a terminal (header pinned, printed as is when it fits), plain when piped.
func page(text string) error {
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		less := exec.Command("less", "-RFS", "--header=1")
		less.Stdin, less.Stdout, less.Stderr = strings.NewReader(text+"\n"), os.Stdout, os.Stderr
		if less.Run() == nil {
			return nil
		}
	}
	_, err := fmt.Println(text)
	return err
}
