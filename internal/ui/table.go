package ui

import (
	"fmt"
	"slices"
	"strings"

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
}

var (
	market   = column{"MARKET", -16, func(m api.Asset) (string, lipgloss.Color) { return m.Pair(), white }}
	kind     = column{"TYPE", -6, func(m api.Asset) (string, lipgloss.Color) { return m.Kind(), dim }}
	leverage = column{"LEVERAGE", 10, perp(func(m api.Asset) (string, lipgloss.Color) { return fmt.Sprintf("%dx", m.MaxLeverage), dim })}
	price    = column{"PRICE", 12, func(m api.Asset) (string, lipgloss.Color) { return px(m, m.MarkPx), white }}
	move     = column{"24H CHANGE", 22, change}
	funding  = column{"8H FUNDING", 12, perp(func(m api.Asset) (string, lipgloss.Color) {
		if m.Funding < 0 {
			return fmt.Sprintf("%.4f%%", m.Funding*800), red
		}
		return fmt.Sprintf("%.4f%%", m.Funding*800), white
	})}
	volume = column{"24H VOLUME", 18, func(m api.Asset) (string, lipgloss.Color) { return num.Sprintf("$%.0f", m.DayNtlVlm), white }}
	mcap   = column{"MARKET CAP", 22, func(m api.Asset) (string, lipgloss.Color) {
		if v := m.Cap(); v > 0 {
			return num.Sprintf("$%.0f", v), white
		}
		return "--", dim
	}}
	interest = column{"OPEN INTEREST", 18, perp(func(m api.Asset) (string, lipgloss.Color) {
		return num.Sprintf("$%.0f", m.OpenInterest*m.MarkPx), white
	})}
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

// Table lists markets like the website's picker; a list of only spot pairs gets its spot columns.
func Table(ms []api.Asset, sel int) string {
	cols := []column{market, price, move, volume, mcap}
	if len(ms) == 0 || slices.ContainsFunc(ms, func(m api.Asset) bool { return !m.Spot }) {
		cols = []column{market, kind, leverage, price, move, funding, volume, interest}
	}
	head := ""
	for _, c := range cols {
		head += fgf(yellow, "%*s", c.w, c.name)
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

// Choices are the picker's rows.
func (s Screen) Choices() []api.Asset { return Filter(s.Markets, Kinds[s.Tab]+" "+s.Query) }

// picker shows the choices in place of the chart and book, scrolled to keep the pick in view.
func (s Screen) picker(w, h int) string {
	ms := s.Choices()
	from := max(0, s.Pick-(h-3))
	ms = ms[from:min(len(ms), from+h-2)]
	return lipgloss.Place(w, h, 0, 0, lipgloss.NewStyle().PaddingLeft(2).Render("\n"+Table(ms, s.Pick-from)))
}
