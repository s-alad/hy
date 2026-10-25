package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"hy/internal/api"
)

// side draws the book over the trades; its left edge is the body's divider, a $ at the spread.
func (s Screen) side(h int) string {
	div := fg(gray, "│") + "  "
	n := min(11, max((h-9)/2, 1)) // up to Hyperliquid's 11 levels a side; trades get the rest
	amount, show := func(l api.Level) float64 { return l.Sz }, s.sz
	if s.USD {
		amount, show = func(l api.Level) float64 { return l.Sz * l.Px }, func(v float64) string { return num.Sprintf("%.0f", v) }
	}
	book := s.Book.Levels
	var tot [2][]float64
	top := 0.0
	for i, lv := range book {
		t := 0.0
		for _, l := range lv[:min(n, len(lv))] {
			t += amount(l)
			tot[i] = append(tot[i], t)
		}
		top = max(top, t)
	}
	// level shades a row by its total like the website, lighting the whole row when the latest change touched it.
	level := func(side, i int) string {
		if i >= len(tot[side]) {
			return div
		}
		l, t := book[side][i], tot[side][i]
		row := fmt.Sprintf("%-10s%12s%14s", s.px(l.Px), show(amount(l)), show(t))
		k, bg := int(math.Ceil(t/top*float64(len(row)))), depth[side]
		if slices.ContainsFunc(s.Fresh.L[side], func(f api.L2Level) bool { return f.P == l.Px }) {
			k, bg = len(row), flash[side]
		}
		out := div
		for j, r := range row {
			st := style(white)
			if j < 10 {
				st = style(tone(side == 0))
			}
			if j < k {
				st = st.Background(bg)
			}
			out += st.Render(string(r))
		}
		return out
	}

	lines := []string{fg(gray, "│"), div + fgf(yellow, "%-10s%12s%14s", "PRICE", "SIZE", "TOTAL")}
	for i := n - 1; i >= 0; i-- {
		lines = append(lines, level(1, i))
	}
	spread := bold(yellow, "$") + "  "
	if bid, ask := s.BBO.Levels[0].Px, s.BBO.Levels[1].Px; bid > 0 && ask > 0 {
		spread += fgf(yellow, "%-10s", "SPREAD") + bold(white, fmt.Sprintf("%12s%13.3f%%", s.px(ask-bid), (ask-bid)/ask*100))
	}
	lines = append(lines, spread)
	for i := range n {
		lines = append(lines, level(0, i))
	}
	lines = append(lines, fg(gray, "├"+strings.Repeat("─", 38)), div+fgf(yellow, "%-10s%12s%14s", "PRICE", "SIZE", "TIME"))
	for _, t := range slices.Backward(s.Trades) {
		lines = append(lines, div+fgf(tone(t.Side == "B"), "%-10s", s.px(t.Px))+fgf(white, "%12s", s.sz(t.Sz))+
			fgf(dim, "%14s", time.UnixMilli(t.Time).Format("15:04:05")))
	}
	for len(lines) < h {
		lines = append(lines, div)
	}
	return strings.Join(lines[:h], "\n")
}
