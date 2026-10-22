package main

import (
	"fmt"
	"os"

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
	_, err = tea.NewProgram(app.New(m, token), tea.WithAltScreen()).Run()
	return err
}
