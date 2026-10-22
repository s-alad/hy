package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/ui"
)

type tick int

type App struct{ ui.Screen }

func New(m api.Asset, token string) *App {
	return &App{ui.Screen{Market: m, Token: token, Interval: 3}} // 1h
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.fetch(true), every(1))
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "1", "2", "3", "4", "5", "6":
			a.Interval = int(k[0] - '1')
			return a, a.fetch(true)
		}
	case tea.WindowSizeMsg:
		a.W, a.H = msg.Width, msg.Height
	case tick:
		return a, tea.Batch(every(msg+1), a.fetch(msg%5 == 0))
	case api.Asset:
		a.Market, a.Err = msg, nil
	case api.L2Book:
		a.Book = msg
	case []api.Trade:
		a.Trades = msg
	case []api.Candle:
		a.Candles = msg
	case error:
		a.Err = msg
	}
	return a, nil
}

// fetch refreshes the book every call and everything else when all is set,
// keeping well under Hyperliquid's 1200 weight/minute limit.
func (a *App) fetch(all bool) tea.Cmd {
	coin, iv := a.Market.Name, api.Intervals[a.Interval]
	cmds := []tea.Cmd{func() tea.Msg { return result(api.Book(coin)) }}
	if all {
		cmds = append(cmds,
			func() tea.Msg { return result(api.Market(coin)) },
			func() tea.Msg { return result(api.Trades(coin)) },
			func() tea.Msg { return result(api.Candles(coin, iv)) },
		)
	}
	return tea.Batch(cmds...)
}

func result[T any](v T, err error) tea.Msg {
	if err != nil {
		return err
	}
	return v
}

func every(n tick) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return n })
}
