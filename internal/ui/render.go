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
	green  = lipgloss.Color("#1fa67d") // Hyperliquid's buy
	red    = lipgloss.Color("#ed7088") // and sell
	white  = lipgloss.Color("#f6fefd")
	dim    = lipgloss.Color("#9aa3a4")
	gray   = lipgloss.Color("#3a4a50")
	tex    = lipgloss.Color("#1e2a2f")
	ink    = lipgloss.Color("#0f1a1f") // Hyperliquid's surface
	picked = lipgloss.Color("#2e3c27") // yellow at 15% over ink

	depth = [2]lipgloss.Color{"#112f2d", "#30272f"} // bids, asks: green and red at 15% over ink
	flash = [2]lipgloss.Color{"#154b40", "#5d3844"} // at 35%

	num = message.NewPrinter(language.English)
)

type Screen struct {
	Settings
	Market  api.Asset
	Book    api.L2Book
	Fresh   api.L2Change // the book's latest change, flashed
	BBO     api.BBO
	Trades  []api.Trade
	Candles []api.Candle
	Rates   []api.Funding
	Markets []api.Asset // for the picker
	Err     error
}

// Settings survive switching markets.
type Settings struct {
	W, H              int
	Interval, Group   int
	FundIv            int // index into api.FundingIntervals
	Funding, USD, Log bool
	Picking           bool
	Query             string
	Pick              int
}

// View stacks header, stats and a body of chart | book+trades between two rules.
// Fixed rows: 2 padding + 6 header + 1 rule + 2 blank + 2 stats + 2 rules + 1 footer = 16.
func (s Screen) View() string {
	w, h := max(s.W-6, 60), max(s.H-16, 8)
	cw := w - 40
	plot := s.chart
	if s.Funding {
		plot = s.funding
	}
	top, body, bottom := rule(s.head(), cw, w, "┬", s.tail()), lipgloss.JoinHorizontal(lipgloss.Top, plot(cw, h), " ", s.side(h)), rule("", cw, w, "┴", "")
	footer := hints("/", "markets", "← →", "interval", "q", "quit")
	if s.Picking {
		top = rule(fg(gray, "─ ")+bold(yellow, "/ ")+bold(white, s.Query)+fg(yellow, "▏ "), w-1, w, "", "")
		body, bottom = s.picker(w, h), rule("", w-1, w, "", "")
		footer = hints("↑ ↓", "select", "⏎", "open", "esc", "back") + fg(dim, "   type to filter")
	}
	if s.Err != nil {
		footer += fg(red, "   "+s.Err.Error())
	}
	return frame(s.H, lipgloss.NewStyle().MaxWidth(w).Render(lipgloss.JoinVertical(lipgloss.Left,
		s.header(w), fg(yellow, strings.Repeat("─", w)), "", s.stats(), "", top, body, bottom, footer)))
}

func (s Screen) header(w int) string {
	name := s.Market.Name
	link := style(yellow).Bold(true).Underline(true).Render("https://app.hyperliquid.xyz/trade/" + name)
	right := lipgloss.NewStyle().Align(lipgloss.Right).Render(strings.Join(append([]string{"", link, ""}, banner(name+"/USDC")...), "\n"))
	gap := strings.Repeat(" ", max(0, w-lipgloss.Width(logo)-lipgloss.Width(right)))
	return bold(yellow, lipgloss.JoinHorizontal(lipgloss.Top, logo, gap, right))
}

func (s Screen) stats() string {
	m := s.Market
	chg, c := change(m)
	next := time.Until(time.Now().Truncate(time.Hour).Add(time.Hour))
	stat := func(label, value string, c lipgloss.Color) string {
		return lipgloss.NewStyle().MarginRight(3).Render(fg(yellow, label) + "\n" + bold(c, value))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		stat("MARK", s.px(m.MarkPx), white),
		stat("ORACLE", s.px(m.OraclePx), white),
		stat("24H CHANGE", chg, c),
		stat("24H VOLUME", num.Sprintf("$%.2f", m.DayNtlVlm), white),
		stat("OPEN INTEREST", num.Sprintf("$%.2f", m.OpenInterest*m.MarkPx), white),
		stat("FUNDING", fmt.Sprintf("%.4f%%", m.Funding*100), white),
		stat("COUNTDOWN", time.Time{}.Add(next).Format("15:04:05"), white),
		stat("MAX LEVERAGE", fmt.Sprintf("%dx", m.MaxLeverage), white),
	)
}

// head is the chart's controls: price or funding, and the price chart's interval and scale.
func (s Screen) head() string {
	out := fg(gray, "─ ") + toggle(!s.Funding, "PRICE") + toggle(s.Funding, "FUNDING") + fg(gray, " ─ ")
	if s.Funding {
		return out + chips(s.FundIv, names(api.FundingIntervals)...) + " "
	}
	return out + chips(s.Interval, names(api.Intervals)...) + fg(gray, " ─ ") + toggle(s.Log, "LOG") + " "
}

func names(ivs []api.Interval) []string {
	var out []string
	for _, iv := range ivs {
		out = append(out, iv.Name)
	}
	return out
}

// tail is the book's controls: price grouping and size unit.
func (s Screen) tail() string {
	unit := s.Market.Name
	if s.USD {
		unit = "USDC"
	}
	return fg(gray, "─ ") + lit("GROUP") + " " + chips(0, step(s.Market.MarkPx, api.Groupings[s.Group])) +
		fg(gray, " ─ ") + lit("UNIT") + " " + chips(0, unit) + " "
}

// toggle shows a switch: a chip when on, else its name with the key letter lit.
func toggle(on bool, name string) string {
	if on {
		return chips(0, name)
	}
	return " " + lit(name) + " "
}

// lit shows a control's name with its first letter, the key that drives it, bracketed: [F]UNDING.
func lit(name string) string { return fg(dim, "[") + bold(yellow, name[:1]) + fg(dim, "]"+name[1:]) }

// hints shows keys and what they do, e.g. "/ markets   q quit".
func hints(kv ...string) string {
	var out []string
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, bold(yellow, kv[i])+fg(dim, " "+kv[i+1]))
	}
	return strings.Join(out, "   ")
}

// chips draws names as controls, highlighting the one at on.
func chips(on int, names ...string) string {
	out := ""
	for i, n := range names {
		if i == on {
			out += lipgloss.NewStyle().Background(yellow).Foreground(ink).Bold(true).Render(" " + n + " ")
		} else {
			out += fg(dim, " "+n+" ")
		}
	}
	return out
}

// rule draws a divider with head on the left and tail after the joint at cw+1.
func rule(head string, cw, w int, joint, tail string) string {
	return head + fg(gray, strings.Repeat("─", max(0, cw+1-lipgloss.Width(head)))+joint) + tail +
		fg(gray, strings.Repeat("─", max(0, w-cw-1-lipgloss.Width(joint)-lipgloss.Width(tail))))
}

func frame(h int, s string) string {
	s = lipgloss.NewStyle().Padding(1, 2).Render(s)
	bar := lipgloss.NewStyle().Background(yellow).Width(2).Height(max(h, lipgloss.Height(s))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, bar, s)
}

// dec is Hyperliquid's price precision: 5 significant figures, at most 6-szDecimals decimals.
func dec(m api.Asset) int {
	d := 6 - m.SzDecimals
	if m.MarkPx > 0 {
		d = min(d, 4-int(math.Floor(math.Log10(m.MarkPx))))
	}
	return max(d, 0)
}

func px(m api.Asset, v float64) string { return strconv.FormatFloat(v, 'f', dec(m), 64) }
func (s Screen) px(v float64) string   { return px(s.Market, v) }
func (s Screen) sz(v float64) string   { return num.Sprintf("%.*f", s.Market.SzDecimals, v) }

// change is m's 24h move, e.g. "+0.001467 / +3.44%", and its color.
func change(m api.Asset) (string, lipgloss.Color) {
	d := m.MarkPx - m.PrevDayPx
	return fmt.Sprintf("%+.*f / %+.2f%%", dec(m), d, d/m.PrevDayPx*100), tone(d >= 0)
}

// step is a grouping's price increment at price p, e.g. 0.000005 or 1000.
func step(p float64, g api.Grouping) string {
	e := int(math.Floor(math.Log10(max(p, 1e-9)))) - g.S + 1
	if e >= 0 {
		return strconv.Itoa(g.M) + strings.Repeat("0", e)
	}
	return "0." + strings.Repeat("0", -e-1) + strconv.Itoa(g.M)
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
