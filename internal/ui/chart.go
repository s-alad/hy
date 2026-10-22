package ui

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var bars = []rune(" ▁▂▃▄▅▆▇█")

// chart draws candles over a volume strip, with a price axis on the left and dates below.
func (s Screen) chart(w, h int) string {
	cs := s.Candles
	if len(cs) == 0 {
		return lipgloss.Place(w, h, 0, 0, "")
	}
	top := 0.0
	for _, c := range cs {
		top = max(top, c.H)
	}
	lw := len(s.px(top*2)) + 1 // *2 leaves room for the padded top label
	pw := w - lw - 1
	cs = cs[max(0, len(cs)-pw):]

	lo, hi, mv := math.Inf(1), 0.0, 0.0
	for _, c := range cs {
		lo, hi, mv = min(lo, c.L), max(hi, c.H), max(mv, c.V)
	}
	pad := max((hi-lo)*0.06, hi*1e-4)
	lo, hi = lo-pad, hi+pad

	vh := h / 5
	ch := h - vh - 3
	y := func(v float64) int { return int(math.Round((hi - v) / (hi - lo) * float64(ch-1))) }
	grid := make([][]string, ch)
	for r := range grid {
		grid[r] = slices.Repeat([]string{" "}, pw)
	}
	for x, c := range cs {
		col := tone(c.C >= c.O)
		for r := y(c.H); r <= y(c.L); r++ {
			grid[r][x] = fg(col, "│")
		}
		for r := y(max(c.O, c.C)); r <= y(min(c.O, c.C)); r++ {
			grid[r][x] = fg(col, "┃")
		}
	}

	var lines []string
	add := func(label, row string) {
		lines = append(lines, fgf(dim, "%*s ", lw-1, label)+fg(gray, "│")+row)
	}
	for r, row := range grid {
		label := ""
		if r%5 == 0 {
			label = s.px(hi - float64(r)/float64(ch-1)*(hi-lo))
		}
		add(label, strings.Join(row, ""))
	}
	add("", "")
	for l := vh - 1; l >= 0; l-- {
		row := ""
		for _, c := range cs {
			if e := int(math.Round(c.V/mv*float64(vh*8))) - l*8; e > 0 {
				row += fg(tone(c.C >= c.O), string(bars[min(e, 8)]))
			} else {
				row += fg(tex, "░")
			}
		}
		add("", row)
	}

	ticks := []rune(strings.Repeat(" ", pw))
	for x := 0; x+11 <= len(cs); x += 16 {
		copy(ticks[x:], []rune(time.UnixMilli(cs[x].Time).Format("01/02 15:04")))
	}
	lines = append(lines,
		strings.Repeat(" ", lw)+fg(gray, "└"+strings.Repeat("─", pw)),
		strings.Repeat(" ", lw+1)+fg(dim, string(ticks)))
	return strings.Join(lines, "\n")
}
