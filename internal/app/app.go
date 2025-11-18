package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/models"
	"hy/internal/ui"
)

type App struct {
	api    *api.Client
	ticker *models.Ticker
	err    error
	width  int
	height int
}

func New() *App {
	return &App{api: api.NewClient()}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			ticker, err := a.api.GetTicker("HYPE")
			if err != nil {
				return models.MarketData{Error: err}
			}
			return models.MarketData{Ticker: ticker}
		},
		tea.WindowSize(),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	case models.MarketData:
		if msg.Error != nil {
			a.err = msg.Error
		} else {
			a.ticker = msg.Ticker
		}
		return a, nil
	}
	return a, nil
}

func (a *App) View() string {
	return ui.Render(a.ticker, a.err, a.width)
}
