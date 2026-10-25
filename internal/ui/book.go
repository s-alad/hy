package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// side draws the order book over recent trades. Its left edge is the body's
// divider, which turns into a $ at the spread.
func (s Screen) side(h int) string {
	div := fg(gray, "│") + "  "
	book := s.Book.Levels
	var tot [2][]float64
	top := 0.0
	for i, lv := range book {
		t := 0.0
		for _, l := range lv {
			t += l.Sz
			tot[i] = append(tot[i], t)
		}
		top = max(top, t)
	}
	level := func(side, i int) string {
		if i >= len(book[side]) {
			return div
		}
		c, t, k := tone(side == 0), tot[side][i], int(math.Round(tot[side][i]/top*4))
		return div + fgf(c, "%-10s", s.px(book[side][i].Px)) + fgf(white, "%10s%11s ", s.sz(book[side][i].Sz), s.sz(t)) +
			fg(c, strings.Repeat("█", k)) + fg(tex, strings.Repeat("░", 4-k))
	}

	n := max((h-4)/4, 1) // book and trades split the height evenly
	lines := []string{div + fgf(yellow, "%-10s%10s%11s", "PRICE", "SIZE", "TOTAL")}
	for i := n - 1; i >= 0; i-- {
		lines = append(lines, level(1, i))
	}
	spread := bold(yellow, "$") + "  "
	if len(book[0]) > 0 && len(book[1]) > 0 {
		d := book[1][0].Px - book[0][0].Px
		spread += fgf(yellow, "%-10s", "SPREAD") + bold(white, fmt.Sprintf("%10s%10.3f%%", s.px(d), d/book[1][0].Px*100))
	}
	lines = append(lines, spread)
	for i := range n {
		lines = append(lines, level(0, i))
	}
	lines = append(lines, fg(gray, "├"+strings.Repeat("─", 38)), div+fgf(yellow, "%-10s%10s%11s", "PRICE", "SIZE", "TIME"))
	for _, t := range slices.Backward(s.Trades) {
		lines = append(lines, div+fgf(tone(t.Side == "B"), "%-10s", s.px(t.Px))+fgf(white, "%10s", s.sz(t.Sz))+
			fgf(dim, "%11s", time.UnixMilli(t.Time).Format("15:04:05")))
	}
	for len(lines) < h {
		lines = append(lines, div)
	}
	return strings.Join(lines[:h], "\n")
}
