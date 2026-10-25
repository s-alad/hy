package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"hy/internal/api"
)

// Table lists markets like Hyperliquid's market picker, highlighting row sel.
func Table(ms []api.Asset, sel int) string {
	lines := []string{fgf(yellow, "%-10s%10s%12s%22s%12s%18s%18s", "COIN", "LEVERAGE", "PRICE", "24H CHANGE", "8H FUNDING", "24H VOLUME", "OPEN INTEREST")}
	for i, m := range ms {
		cell := func(c lipgloss.Color, format string, a ...any) string {
			st := style(c)
			if i == sel {
				st = st.Background(picked).Bold(true)
			}
			return st.Render(fmt.Sprintf(format, a...))
		}
		name, funding := white, white
		if i == sel {
			name = yellow
		}
		if m.Funding < 0 {
			funding = red
		}
		chg, c := change(m)
		lines = append(lines, cell(name, "%-10s", m.Name)+cell(dim, "%9dx", m.MaxLeverage)+cell(white, "%12s", px(m, m.MarkPx))+
			cell(c, "%22s", chg)+cell(funding, "%11.4f%%", m.Funding*800)+
			cell(white, "%18s", num.Sprintf("$%.0f", m.DayNtlVlm))+cell(white, "%18s", num.Sprintf("$%.0f", m.OpenInterest*m.MarkPx)))
	}
	return strings.Join(lines, "\n")
}

// Filter keeps the markets whose name contains q, ignoring case.
func Filter(ms []api.Asset, q string) []api.Asset {
	return slices.DeleteFunc(slices.Clone(ms), func(m api.Asset) bool {
		return !strings.Contains(strings.ToLower(m.Name), strings.ToLower(q))
	})
}

// picker shows the filtered markets in place of the chart and book, scrolled to keep the pick in view.
func (s Screen) picker(w, h int) string {
	ms := Filter(s.Markets, s.Query)
	from := max(0, s.Pick-(h-3))
	ms = ms[from:min(len(ms), from+h-2)]
	return lipgloss.Place(w, h, 0, 0, lipgloss.NewStyle().PaddingLeft(2).Render("\n"+Table(ms, s.Pick-from)))
}
