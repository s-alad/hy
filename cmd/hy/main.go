package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/app"
)

func main() {
	a := app.New()
	p := tea.NewProgram(a, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}
