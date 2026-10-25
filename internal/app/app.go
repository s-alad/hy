package app

import (
	"cmp"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"hy/internal/api"
	"hy/internal/ui"
)

type App struct {
	ui.Screen
	stream *api.Stream
	rated  time.Time // when funding history was last requested
}

// New shows market m, or the markets list filtered by q when m is empty.
func New(m api.Asset, ms []api.Asset, q string, s *api.Stream) *App {
	return &App{Screen: ui.Screen{Market: m, Markets: ms, Settings: ui.Settings{Interval: 5, FundIv: 1, Picking: m.Name == "", Query: q}}, stream: s} // 1h candles, 8h funding
}

func (a *App) Init() tea.Cmd {
	if a.Listing() {
		a.watch() // just the list's feeds
		return nil
	}
	return a.open(a.Market)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c := api.Coin(msg); c != "" && c != a.Market.Name {
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
		cmd := a.rates() // each hour's funding as it lands
		if a.Err != nil {
			cmd = tea.Batch(a.history(), cmd) // back after a drop: refill what we missed
		}
		a.Market.AssetCtx, a.Err = msg.Ctx, nil
		return a, cmd
	case api.Ctxs:
		msg.Apply(a.Markets)
	case api.L2Book:
		a.Book = msg
	case api.BBO:
		a.BBO = msg
	case api.L2Change:
		a.Book, a.Fresh = a.Book.Apply(msg), msg
	case []api.Trade: // a reconnect resends the last 30, so skip ones we have
		for _, t := range msg {
			if !slices.ContainsFunc(a.Trades, func(o api.Trade) bool { return o.Tid == t.Tid }) {
				a.Trades = append(a.Trades, t)
			}
		}
		slices.SortStableFunc(a.Trades, func(x, y api.Trade) int { return cmp.Compare(x.Time, y.Time) })
		a.Trades = a.Trades[max(0, len(a.Trades)-100):]
	case []api.Candle:
		if len(msg) > 0 && msg[0].I == iv {
			a.Candles = msg
		}
	case api.Candle:
		if msg.I == iv {
			if i := slices.IndexFunc(a.Candles, func(c api.Candle) bool { return c.Time == msg.Time }); i >= 0 {
				a.Candles[i] = msg
			} else {
				a.Candles = append(a.Candles, msg)
			}
		}
	case []api.Funding:
		a.Rates = msg
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
		d := map[string]int{"left": -1, "right": 1}[k]
		if a.Funding {
			a.FundIv = max(0, min(len(api.FundingIntervals)-1, a.FundIv+d))
			return nil
		}
		i := max(0, min(len(api.Intervals)-1, a.Interval+d))
		if i == a.Interval {
			return nil
		}
		a.Interval, a.Candles = i, nil
		a.watch()
		return a.history()
	case "f":
		a.Funding = !a.Funding && !a.Market.Spot // spot has no funding
		return a.rates()
	case "p":
		a.Funding = false
	case "g":
		a.Group = (a.Group + 1) % len(api.Groupings)
		a.watch()
	case "u":
		a.USD = !a.USD
	case "l":
		a.Log = !a.Log
	case "/":
		a.Picking, a.Query, a.Pick = true, "", ""
	}
	return nil
}

func (a *App) pick(k tea.KeyMsg) tea.Cmd {
	ms, i := a.Picked()
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEsc:
		if a.Listing() {
			return tea.Quit
		}
		a.Picking = false
	case tea.KeyEnter:
		if len(ms) > 0 {
			return a.open(ms[i])
		}
	case tea.KeyTab:
		a.Tab, a.Pick = (a.Tab+1)%len(ui.Kinds), ""
	case tea.KeyLeft, tea.KeyRight:
		a.Sort = a.NextSort(map[tea.KeyType]int{tea.KeyLeft: -1, tea.KeyRight: 1}[k.Type])
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
		a.Pick = at(ms, i+map[tea.KeyType]int{tea.KeyUp: -1, tea.KeyDown: 1, tea.KeyPgUp: -a.Rows(), tea.KeyPgDown: a.Rows()}[k.Type])
	case tea.KeyBackspace:
		if q := []rune(a.Query); len(q) > 0 {
			a.Query, a.Pick = string(q[:len(q)-1]), ""
		}
	case tea.KeyRunes:
		a.Query, a.Pick = a.Query+string(k.Runes), ""
	}
	return nil
}

// at names the market at i, kept within ms.
func at(ms []api.Asset, i int) string {
	if len(ms) == 0 {
		return ""
	}
	return ms[max(0, min(i, len(ms)-1))].Name
}

func (a *App) open(m api.Asset) tea.Cmd {
	a.Screen, a.rated = ui.Screen{Settings: a.Settings, Market: m, Markets: a.Markets}, time.Time{}
	a.Picking, a.Funding = false, a.Funding && !m.Spot
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

// rates fetches ~100 days of funding when it's missing or the hour's is due
// (funding lands on the hour; we look from :01), asking at most once a minute.
func (a *App) rates() tea.Cmd {
	due := time.Now().Add(-time.Minute).Truncate(time.Hour).UnixMilli()
	if n := len(a.Rates); !a.Funding || time.Since(a.rated) < time.Minute || n > 0 && a.Rates[n-1].Time >= due {
		return nil
	}
	a.rated = time.Now()
	coin := a.Market.Name
	return func() tea.Msg { return result(api.FundingHistory(coin, time.Now().Add(-2400*time.Hour))) }
}

func result[T any](v T, err error) tea.Msg {
	if err != nil {
		return err
	}
	return v
}
