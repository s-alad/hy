package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"hy/internal/api"
)

var bars = []rune(" ▁▂▃▄▅▆▇█")

// chart draws candles over a volume strip, tagging the last price on the axis.
func (s Screen) chart(w, h int) string {
	cs := s.Candles
	if len(cs) == 0 {
		return lipgloss.Place(w, h, 0, 0, "")
	}
	peak := 0.0
	for _, c := range cs {
		peak = max(peak, c.H)
	}
	lw := len(s.px(peak*2)) + 2 // room for the padded top label and the last price's tag
	pw := w - lw - 1
	cs = cs[max(0, len(cs)-pw):]

	f, inv := func(v float64) float64 { return v }, func(v float64) float64 { return v }
	if s.Log {
		f, inv = math.Log, math.Exp
	}
	lo, hi, mv := math.Inf(1), math.Inf(-1), 0.0
	for _, c := range cs {
		lo, hi, mv = min(lo, f(c.L)), max(hi, f(c.H)), max(mv, c.V)
	}
	pad := (hi-lo)*0.06 + 1e-9
	lo, hi = lo-pad, hi+pad

	vh := h / 5
	ch := h - vh - 5 // spacer, readout, gap, axis and times take the rest
	y := func(v float64) int { return int(math.Round((hi - f(v)) / (hi - lo) * float64(ch-1))) }
	g := grid(ch, pw)
	for x, c := range cs {
		col := tone(c.C >= c.O)
		for r := y(c.H); r <= y(c.L); r++ {
			g[r][x] = fg(col, "│")
		}
		for r := y(max(c.O, c.C)); r <= y(min(c.O, c.C)); r++ {
			g[r][x] = fg(col, "┃")
		}
	}

	last := cs[len(cs)-1]
	up, lr := last.C >= last.O, y(last.C)
	dots := flash[1]
	if up {
		dots = flash[0]
	}
	for x, cell := range g[lr] {
		if cell == " " {
			g[lr][x] = fg(dots, "┈")
		}
	}
	labels := make([]string, ch)
	for _, v := range steps(inv(lo), inv(hi), ch) {
		if r := y(v); (r-lr)*(r-lr) > 1 {
			labels[r] = fgf(dim, "%*s ", lw-1, s.px(v))
		}
	}
	tag := " " + s.px(last.C) + " "
	labels[lr] = strings.Repeat(" ", max(0, lw-len(tag))) + lipgloss.NewStyle().Background(tone(up)).Foreground(ink).Bold(true).Render(tag)

	rows := append(join(g), "")
	for l := vh - 1; l >= 0; l-- {
		row := ""
		for _, c := range cs {
			if e := int(math.Round(c.V/mv*float64(vh*8))) - l*8; e > 0 {
				row += fg(tone(c.C >= c.O), string(bars[min(e, 8)]))
			} else {
				row += fg(tex, "░")
			}
		}
		rows = append(rows, row)
	}
	times := make([]int64, len(cs))
	for i, c := range cs {
		times[i] = c.Time
	}
	d := last.C - last.O
	head := readout("O", s.px(last.O), "H", s.px(last.H), "L", s.px(last.L), "C", s.px(last.C)) +
		fgf(tone(d >= 0), "%+.*f (%+.2f%%)", dec(s.Market), d, d/last.O*100)
	return axes(head, labels, rows, times, api.Intervals[s.Interval].Dur, lw, pw)
}

// funding draws hourly funding as a step line around zero.
func (s Screen) funding(w, h int) string {
	rs := s.Rates
	if len(rs) == 0 {
		return lipgloss.Place(w, h, 0, 0, "")
	}
	lw := 10
	pw := w - lw - 1
	rs = rs[max(0, len(rs)-pw):]
	lo, hi, sum := 0.0, 0.0, 0.0
	for _, r := range rs {
		lo, hi, sum = min(lo, r.FundingRate), max(hi, r.FundingRate), sum+r.FundingRate
	}
	pad := (hi-lo)*0.15 + 1e-12
	lo, hi = lo-pad, hi+pad

	ch := h - 4 // spacer, readout, axis and times take the rest
	y := func(v float64) int { return int(math.Round((hi - v) / (hi - lo) * float64(ch-1))) }
	g, zero := grid(ch, pw), y(0)
	for x := range g[zero] {
		g[zero][x] = fg(gray, "┈")
	}
	for x, r := range rs {
		c, now, was := tone(r.FundingRate >= 0), y(r.FundingRate), y(rs[max(x-1, 0)].FundingRate)
		if now == was {
			g[now][x] = fg(c, "─")
			continue
		}
		from, to := "╮", "╰" // falling
		if now < was {
			from, to = "╯", "╭"
		}
		g[was][x], g[now][x] = fg(c, from), fg(c, to)
		for k := min(now, was) + 1; k < max(now, was); k++ {
			g[k][x] = fg(c, "│")
		}
	}
	labels := make([]string, ch)
	for _, v := range steps(lo, hi, ch) {
		if r := y(v); (r-zero)*(r-zero) > 1 {
			labels[r] = fgf(dim, "%*.4f%% ", lw-2, v*100)
		}
	}
	labels[zero] = fgf(dim, "%*s ", lw-1, "0%")
	times := make([]int64, len(rs))
	for i, r := range rs {
		times[i] = r.Time
	}
	now, pct := s.Market.Funding*100, func(v float64) string { return fmt.Sprintf("%.4f%%", v) }
	head := readout("NOW", pct(now), "8H", pct(now*8), "APR", fmt.Sprintf("%.2f%%", now*24*365), "AVG", pct(sum/float64(len(rs))*100))
	return axes(head, labels, join(g), times, time.Hour, lw, pw)
}

// steps returns round values 1, 2 or 5 × 10^k apart between lo and hi, about one per four rows.
func steps(lo, hi float64, rows int) []float64 {
	raw := (hi - lo) / max(float64(rows)/4, 1)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := mag
	for _, m := range []float64{2, 5, 10} {
		if math.Abs(m*mag-raw) < math.Abs(step-raw) {
			step = m * mag
		}
	}
	var vs []float64
	for i := math.Ceil(lo / step); i*step <= hi; i++ {
		vs = append(vs, i*step)
	}
	return vs
}

// axes frames plot rows: a spacer and readout above, labels on a left axis, and below,
// times at the first candle of each round period (a day for 1h candles, like Hyperliquid).
func axes(head string, labels, rows []string, times []int64, iv time.Duration, lw, pw int) string {
	pad := strings.Repeat(" ", lw)
	lines := []string{pad + fg(gray, "│"), pad + fg(gray, "│") + " " + head}
	for i, row := range rows {
		label := pad
		if i < len(labels) && labels[i] != "" {
			label = labels[i]
		}
		lines = append(lines, label+fg(gray, "│")+row)
	}
	every := 16 * iv
	for _, d := range []time.Duration{15, 30, 60, 180, 360, 720, 1440, 2880, 10080, 20160, 40320} {
		if d*time.Minute >= every {
			every = d * time.Minute
			break
		}
	}
	layout, loc := "01/02 15:04", time.Local
	if every >= 24*time.Hour {
		layout = "Jan 02"
	}
	if iv >= 24*time.Hour {
		loc = time.UTC // daily candles open at UTC midnight
	}
	if iv >= 168*time.Hour {
		layout = "Jan 2006"
	}
	_, off := time.Now().In(loc).Zone()
	span, shift := every.Milliseconds(), int64(off)*1000
	ticks, next := []rune(strings.Repeat(" ", pw)), 0
	for x := 1; x < len(times); x++ {
		if (times[x]+shift)/span != (times[x-1]+shift)/span && x >= next && x+len(layout) <= pw {
			copy(ticks[x:], []rune(time.UnixMilli(times[x]).In(loc).Format(layout)))
			next = x + len(layout) + 2
		}
	}
	return strings.Join(append(lines, pad+fg(gray, "└"+strings.Repeat("─", pw)), pad+" "+fg(dim, string(ticks))), "\n")
}

// readout pairs labels with values, e.g. "O 80890  H 81014  ".
func readout(kv ...string) string {
	out := ""
	for i := 0; i+1 < len(kv); i += 2 {
		out += fg(yellow, kv[i]+" ") + fg(white, kv[i+1]) + "  "
	}
	return out
}

func grid(rows, cols int) [][]string {
	g := make([][]string, rows)
	for r := range g {
		g[r] = slices.Repeat([]string{" "}, cols)
	}
	return g
}

func join(g [][]string) []string {
	rows := make([]string, len(g))
	for r, cells := range g {
		rows[r] = strings.Join(cells, "")
	}
	return rows
}
