package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"hy/internal/api"
)

var (
	yellow = lipgloss.Color("#dcfe52")
	green  = lipgloss.Color("#5af78e")
	red    = lipgloss.Color("#f85149")
	white  = lipgloss.Color("#e6edf3")
	dim    = lipgloss.Color("#7d8790")
	gray   = lipgloss.Color("#3f474f")
	tex    = lipgloss.Color("#262c33")
	ink    = lipgloss.Color("#0c0f12")

	num = message.NewPrinter(language.English)
)

type Screen struct {
	Market   api.Asset
	Token    string
	Book     api.L2Book
	Trades   []api.Trade
	Candles  []api.Candle
	Interval int
	Err      error
	W, H     int
}

// View stacks header, stats and a body of chart | book+trades between two rules.
// Fixed rows: 2 padding + 6 header + 1 rule + 2 blank + 2 stats + 2 rules + 1 footer = 16.
func (s Screen) View() string {
	w, h := max(s.W-6, 60), max(s.H-16, 8)
	cw := w - 40
	footer := fg(dim, "1-6 interval   q quit")
	if s.Err != nil {
		footer += fg(red, "   "+s.Err.Error())
	}
	return frame(s.H, lipgloss.NewStyle().MaxWidth(w).Render(lipgloss.JoinVertical(lipgloss.Left,
		s.header(w),
		fg(yellow, strings.Repeat("─", w)),
		"",
		s.stats(),
		"",
		rule(s.chips(), cw, w, "┬"),
		lipgloss.JoinHorizontal(lipgloss.Top, s.chart(cw, h), " ", s.side(h)),
		rule("", cw, w, "┴"),
		footer,
	)))
}

func (s Screen) header(w int) string {
	name := s.Market.Name
	right := lipgloss.NewStyle().Align(lipgloss.Right).Render(strings.Join(append(
		[]string{bits(name), lipgloss.NewStyle().Underline(true).Render(s.Token), ""},
		banner(name+"/USDC")...), "\n"))
	gap := strings.Repeat(" ", max(0, w-lipgloss.Width(logo)-lipgloss.Width(right)))
	return bold(yellow, lipgloss.JoinHorizontal(lipgloss.Top, logo, gap, right))
}

func (s Screen) stats() string {
	m := s.Market
	chg := m.MarkPx - m.PrevDayPx
	next := time.Until(time.Now().Truncate(time.Hour).Add(time.Hour))
	stat := func(label, value string, c lipgloss.Color) string {
		return lipgloss.NewStyle().MarginRight(3).Render(fg(yellow, label) + "\n" + bold(c, value))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		stat("MARK", s.px(m.MarkPx), white),
		stat("ORACLE", s.px(m.OraclePx), white),
		stat("24H CHANGE", fmt.Sprintf("%+.*f / %+.2f%%", s.dec(), chg, chg/m.PrevDayPx*100), tone(chg >= 0)),
		stat("24H VOLUME", num.Sprintf("$%.2f", m.DayNtlVlm), white),
		stat("OPEN INTEREST", num.Sprintf("$%.2f", m.OpenInterest*m.MarkPx), white),
		stat("FUNDING", fmt.Sprintf("%.4f%%", m.Funding*100), white),
		stat("COUNTDOWN", time.Time{}.Add(next).Format("15:04:05"), white),
	)
}

func (s Screen) chips() string {
	out := fg(gray, "─ ")
	for i, iv := range api.Intervals {
		if i == s.Interval {
			out += lipgloss.NewStyle().Background(yellow).Foreground(ink).Bold(true).Render(" " + iv.Name + " ")
		} else {
			out += fg(dim, " "+iv.Name+" ")
		}
	}
	return out + " "
}

// dec is Hyperliquid's price precision: 5 significant figures, at most 6-szDecimals decimals.
func (s Screen) dec() int {
	d := 6 - s.Market.SzDecimals
	if p := s.Market.MarkPx; p > 0 {
		d = min(d, 4-int(math.Floor(math.Log10(p))))
	}
	return max(d, 0)
}

func (s Screen) px(v float64) string { return strconv.FormatFloat(v, 'f', s.dec(), 64) }
func (s Screen) sz(v float64) string { return num.Sprintf("%.*f", s.Market.SzDecimals, v) }

func rule(head string, cw, w int, joint string) string {
	return head + fg(gray, strings.Repeat("─", max(0, cw+1-lipgloss.Width(head)))+joint+strings.Repeat("─", w-cw-2))
}

func frame(h int, s string) string {
	s = lipgloss.NewStyle().Padding(1, 2).Render(s)
	bar := lipgloss.NewStyle().Background(yellow).Width(2).Height(max(h, lipgloss.Height(s))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, bar, s)
}

func bits(s string) string {
	b := make([]string, len(s))
	for i := range len(s) {
		b[i] = fmt.Sprintf("%08b", s[i])
	}
	return strings.Join(b, " ")
}

func tone(up bool) lipgloss.Color {
	if up {
		return green
	}
	return red
}

func style(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func fg(c lipgloss.Color, s string) string                 { return style(c).Render(s) }
func bold(c lipgloss.Color, s string) string               { return style(c).Bold(true).Render(s) }
func fgf(c lipgloss.Color, format string, a ...any) string { return fg(c, fmt.Sprintf(format, a...)) }
