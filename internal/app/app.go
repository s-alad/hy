package app

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/ui"
)

type App struct{ ui.Screen }

func New(m api.Asset, token string) *App {
	return &App{ui.Screen{Market: m, Token: token, Interval: 3}} // 1h
}

func (a *App) Init() tea.Cmd { return a.history() }

// Update takes key presses and api.Watch's stream; only candle history is fetched.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	iv := api.Intervals[a.Interval].Name
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "1", "2", "3", "4", "5", "6":
			a.Interval, a.Candles = int(k[0]-'1'), nil
			return a, a.history()
		}
	case tea.WindowSizeMsg:
		a.W, a.H = msg.Width, msg.Height
	case api.ActiveAssetCtx:
		a.Market.AssetCtx, a.Err = msg.Ctx, nil
	case api.L2Book:
		a.Book = msg
	case api.L2Change:
		a.Book = a.Book.Apply(msg)
	case []api.Trade:
		a.Trades = append(a.Trades, msg...)
		a.Trades = a.Trades[max(0, len(a.Trades)-100):]
	case []api.Candle:
		if len(msg) > 0 && msg[0].I == iv {
			a.Candles = msg
		}
	case api.Candle:
		if msg.I == iv {
			a.Candles = append(slices.DeleteFunc(a.Candles, func(c api.Candle) bool { return c.Time == msg.Time }), msg)
		}
	case error:
		a.Err = msg
	}
	return a, nil
}

func (a *App) history() tea.Cmd {
	coin, iv := a.Market.Name, api.Intervals[a.Interval]
	return func() tea.Msg {
		cs, err := api.Candles(coin, iv)
		if err != nil {
			return err
		}
		return cs
	}
}
