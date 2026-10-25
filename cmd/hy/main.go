package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hy:", err)
		os.Exit(1)
	}
}

func run() error {
	coin := "HYPE"
	if len(os.Args) > 1 {
		coin = os.Args[1]
	}
	m, err := api.Market(coin)
	if err != nil {
		return err
	}
	token, _ := api.Token(m.Name)
	p := tea.NewProgram(app.New(m, token), tea.WithAltScreen())
	go func() {
		for {
			p.Send(api.Watch(m.Name, func(v any) { p.Send(v) }))
			time.Sleep(3 * time.Second) // stays under 30 new connections a minute
		}
	}()
	_, err = p.Run()
	return err
}
