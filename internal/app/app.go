package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/models"
	"hy/internal/ui"
)

type App struct {
	api      *api.Client
	ticker   *models.Ticker
	chart    ui.ChartModel
	err      error
	width    int
	height   int
	interval string
}

func New() *App {
	return &App{
		api:      api.NewClient(),
		chart:    ui.NewChartModel(60, 20), // Start with reasonable defaults
		interval: "1h",
	}
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
		func() tea.Msg {
			candles, err := a.api.GetCandles("HYPE", "1h")
			if err != nil {
				return models.MarketData{Error: err}
			}
			return models.MarketData{Candles: candles}
		},
		tea.WindowSize(),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "1":
			a.interval = "1m"
			return a, a.fetchCandles()
		case "2":
			a.interval = "5m"
			return a, a.fetchCandles()
		case "3":
			a.interval = "15m"
			return a, a.fetchCandles()
		case "4":
			a.interval = "1h"
			return a, a.fetchCandles()
		case "5":
			a.interval = "4h"
			return a, a.fetchCandles()
		case "6":
			a.interval = "1d"
			return a, a.fetchCandles()
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		// Calculate available space for chart
		// Reserve space for left sidebar stats (approx 30-35 chars wide)
		// and header (approx 10-12 lines tall)
		sidebarWidth := 35
		headerHeight := 12

		chartWidth := a.width - sidebarWidth - 4 // -4 for padding
		chartHeight := a.height - headerHeight

		if chartWidth < 20 {
			chartWidth = 20
		}
		if chartHeight < 10 {
			chartHeight = 10
		}

		a.chart.Resize(chartWidth, chartHeight)

	case models.MarketData:
		if msg.Error != nil {
			a.err = msg.Error
		}
		if msg.Ticker != nil {
			a.ticker = msg.Ticker
		}
		if msg.Candles != nil {
			a.chart.SetCandles(msg.Candles)
		}
	}

	a.chart, cmd = a.chart.Update(msg)
	return a, cmd
}

func (a *App) fetchCandles() tea.Cmd {
	return func() tea.Msg {
		candles, err := a.api.GetCandles("HYPE", a.interval)
		if err != nil {
			return models.MarketData{Error: err}
		}
		return models.MarketData{Candles: candles}
	}
}

func (a *App) View() string {
	return ui.Render(a.ticker, &a.chart, a.err, a.width, a.interval)
}
