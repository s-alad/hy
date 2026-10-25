package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"hy/internal/api"
)

// Kinds are the picker's tabs; each is also a word Filter understands.
var Kinds = []string{"ALL", "PERPS", "SPOT", "HIP-3"}

type cell = func(api.Asset) (string, lipgloss.Color)

type column struct {
	name string
	w    int // negative to left-align
	cell cell
	key  func(api.Asset) float64 // what it sorts by, biggest first; nil when it doesn't sort
}

var (
	market   = column{"MARKET", -16, func(m api.Asset) (string, lipgloss.Color) { return m.Pair(), white }, nil}
	kind     = column{"TYPE", -6, func(m api.Asset) (string, lipgloss.Color) { return m.Kind(), dim }, nil}
	leverage = column{"LEVERAGE", 10, perp(func(m api.Asset) (string, lipgloss.Color) { return fmt.Sprintf("%dx", m.MaxLeverage), dim }), func(m api.Asset) float64 { return float64(m.MaxLeverage) }}
	price    = column{"PRICE", 12, func(m api.Asset) (string, lipgloss.Color) { return px(m, m.MarkPx), white }, func(m api.Asset) float64 { return m.MarkPx }}
	move     = column{"24H CHANGE", 23, change, pct}
	funding  = column{"8H FUNDING", 12, perp(func(m api.Asset) (string, lipgloss.Color) {
		if m.Funding < 0 {
			return fmt.Sprintf("%.4f%%", m.Funding*800), red
		}
		return fmt.Sprintf("%.4f%%", m.Funding*800), white
	}), func(m api.Asset) float64 { return m.Funding }}
	volume = column{"24H VOLUME", 18, func(m api.Asset) (string, lipgloss.Color) { return num.Sprintf("$%.0f", m.DayNtlVlm), white }, func(m api.Asset) float64 { return m.DayNtlVlm }}
	mcap   = column{"MARKET CAP", 20, func(m api.Asset) (string, lipgloss.Color) {
		if v := m.Cap(); v > 0 {
			return num.Sprintf("$%.0f", v), white
		}
		return "--", dim
	}, api.Asset.Cap}
	interest = column{"OPEN INTEREST", 18, perp(func(m api.Asset) (string, lipgloss.Color) {
		return num.Sprintf("$%.0f", m.OpenInterest*m.MarkPx), white
	}), func(m api.Asset) float64 { return m.OpenInterest * m.MarkPx }}
)

// perp shows -- for spot markets, which have no leverage, funding or open interest.
func perp(f cell) cell {
	return func(m api.Asset) (string, lipgloss.Color) {
		if m.Spot {
			return "--", dim
		}
		return f(m)
	}
}

// columns are the table's; a list of only spot pairs gets just spot's.
func columns(ms []api.Asset) []column {
	if len(ms) > 0 && !slices.ContainsFunc(ms, func(m api.Asset) bool { return !m.Spot }) {
		return []column{market, price, move, volume, mcap}
	}
	return []column{market, kind, leverage, price, move, funding, volume, interest, mcap}
}

// Table lists markets like the website's picker, the column they're sorted by as a chip.
func Table(ms []api.Asset, sel int, by string) string { return table(columns(ms), ms, sel, by) }

func table(cols []column, ms []api.Asset, sel int, by string) string {
	head := ""
	for _, c := range cols {
		if c.name == by {
			head += strings.Repeat(" ", c.w-len(c.name)-2) + chips(0, c.name)
		} else {
			head += fg(yellow, fmt.Sprintf("%*s", c.w, c.name))
		}
	}
	lines := []string{head}
	for i, m := range ms {
		row := ""
		for j, c := range cols {
			v, col := c.cell(m)
			st := style(col)
			if i == sel {
				st = st.Background(picked).Bold(true)
				if j == 0 {
					st = st.Foreground(yellow)
				}
			}
			if c.name == price.name && m.Tick != 0 && time.Since(m.Ticked) < time.Second { // a tick flashes the price: bids' green up, asks' red down
				row += st.Render(strings.Repeat(" ", c.w-len(v))) + st.Background(flash[max(0, -m.Tick)]).Bold(true).Render(v)
				continue
			}
			row += st.Render(fmt.Sprintf("%*s", c.w, v))
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// Filter keeps the markets matching every word of q: a kind (perps, spot, hip3, a dex) or part of a name.
func Filter(ms []api.Asset, q string) []api.Asset {
	return slices.DeleteFunc(slices.Clone(ms), func(m api.Asset) bool {
		for _, w := range strings.Fields(strings.ToLower(q)) {
			if !matches(m, w) {
				return true
			}
		}
		return false
	})
}

func matches(m api.Asset, w string) bool {
	switch w {
	case "all":
		return true
	case "perp", "perps":
		return !m.Spot && m.Dex == ""
	case "spot":
		return m.Spot
	case "hip3", "hip-3":
		return m.Dex != ""
	}
	return w == m.Dex || strings.Contains(strings.ToLower(m.Pair()+" "+m.Name), w)
}

// by is the column the list is sorted by: volume unless another was chosen.
func (s Screen) by() string { return cmp.Or(s.Sort, volume.name) }

// Choices are the picker's rows: the markets matching its tab and query, sorted.
func (s Screen) Choices() []api.Asset {
	ms := Filter(s.Markets, Kinds[s.Tab]+" "+s.Query)
	for _, c := range columns(ms) {
		if c.name == s.by() && c.key != nil {
			slices.SortStableFunc(ms, func(a, b api.Asset) int { return cmp.Compare(c.key(b), c.key(a)) })
		}
	}
	return ms
}

// Picked returns the picker's rows and the picked one's index, 0 once it's gone.
func (s Screen) Picked() ([]api.Asset, int) {
	ms := s.Choices()
	return ms, max(0, slices.IndexFunc(ms, func(m api.Asset) bool { return m.Name == s.Pick }))
}

// NextSort names the sortable column d places from the current one.
func (s Screen) NextSort(d int) string {
	var names []string
	for _, c := range columns(s.Choices()) {
		if c.key != nil {
			names = append(names, c.name)
		}
	}
	i := slices.Index(names, s.by())
	return names[max(0, min(len(names)-1, i+d))]
}

// picker shows the choices in place of the chart and book, scrolled to keep the pick in view.
func (s Screen) picker(w, h int) string {
	ms, i := s.Picked()
	from := max(0, i-(h-3)) // the window's first row; the columns suit the whole list, not the window
	return lipgloss.Place(w, h, 0, 0, lipgloss.NewStyle().PaddingLeft(2).Render("\n"+table(columns(ms), ms[from:min(len(ms), from+h-2)], i-from, s.by())))
}
