package app

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/ui"
)

type App struct {
	ui.Screen
	stream *api.Stream
}

func New(m api.Asset, s *api.Stream) *App {
	return &App{ui.Screen{Market: m, Settings: ui.Settings{Interval: 5}}, s} // 1h
}

func (a *App) Init() tea.Cmd { return a.open(a.Market) }

// Update takes key presses, the stream and fetched data.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c := api.CoinOf(msg); c != "" && c != a.Market.Name {
		return a, nil // left over from the previous market
	}
	iv := api.Intervals[a.Interval].Name
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if a.Picking {
			return a, a.pick(msg)
		}
		return a, a.key(msg.String())
	case tea.WindowSizeMsg:
		a.W, a.H = msg.Width, msg.Height
	case api.ActiveAssetCtx:
		a.Market.AssetCtx, a.Err = msg.Ctx, nil
	case api.L2Book:
		a.Book = msg
	case api.L2Change:
		a.Book, a.Fresh = a.Book.Apply(msg), msg
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
	case []api.Funding:
		a.Rates = msg
	case []api.Asset:
		a.Markets = msg
	case error:
		a.Err = msg
	}
	return a, nil
}

func (a *App) key(k string) tea.Cmd {
	switch k {
	case "q", "ctrl+c":
		return tea.Quit
	case "left", "right":
		i := max(0, min(len(api.Intervals)-1, a.Interval+map[string]int{"left": -1, "right": 1}[k]))
		if i == a.Interval {
			return nil
		}
		a.Interval, a.Candles = i, nil
		a.watch()
		return a.history()
	case "f":
		a.Funding = !a.Funding
		return a.rates()
	case "g":
		a.Group = (a.Group + 1) % len(api.Groupings)
		a.watch()
	case "u":
		a.USD = !a.USD
	case "l":
		a.Log = !a.Log
	case "/":
		a.Picking, a.Query, a.Pick = true, "", 0
		return func() tea.Msg { return result(api.Markets()) }
	}
	return nil
}

// pick drives the market picker: type to filter, arrows to move, enter to open.
func (a *App) pick(k tea.KeyMsg) tea.Cmd {
	ms := ui.Filter(a.Markets, a.Query)
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEsc:
		a.Picking = false
	case tea.KeyEnter:
		if a.Pick < len(ms) {
			return a.open(ms[a.Pick])
		}
	case tea.KeyUp:
		a.Pick = max(a.Pick-1, 0)
	case tea.KeyDown:
		a.Pick = max(min(a.Pick+1, len(ms)-1), 0)
	case tea.KeyBackspace:
		if q := []rune(a.Query); len(q) > 0 {
			a.Query, a.Pick = string(q[:len(q)-1]), 0
		}
	case tea.KeyRunes:
		a.Query, a.Pick = a.Query+string(k.Runes), 0
	}
	return nil
}

// open switches to market m, keeping the view settings.
func (a *App) open(m api.Asset) tea.Cmd {
	a.Screen = ui.Screen{Settings: a.Settings, Market: m}
	a.Picking = false
	a.watch()
	return tea.Batch(a.history(), a.rates())
}

func (a *App) watch() {
	a.stream.Watch(a.Market.Name, api.Intervals[a.Interval], api.Groupings[a.Group])
}

func (a *App) history() tea.Cmd {
	coin, iv := a.Market.Name, api.Intervals[a.Interval]
	return func() tea.Msg { return result(api.Candles(coin, iv)) }
}

func (a *App) rates() tea.Cmd {
	if !a.Funding {
		return nil
	}
	coin := a.Market.Name
	return func() tea.Msg { return result(api.FundingHistory(coin)) }
}

func result[T any](v T, err error) tea.Msg {
	if err != nil {
		return err
	}
	return v
}
