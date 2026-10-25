package api

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

type req map[string]any

type AssetCtx struct {
	MarkPx, OraclePx, PrevDayPx, Funding, OpenInterest, DayNtlVlm float64 `json:",string"`
}

type Asset struct {
	Name        string
	SzDecimals  int
	MaxLeverage int
	IsDelisted  bool
	AssetCtx
}

type ActiveAssetCtx struct {
	Coin string
	Ctx  AssetCtx
}

type Level struct {
	Px, Sz float64 `json:",string"`
}

type L2Book struct {
	Coin   string
	Levels [2][]Level // bids, asks
}

// BBO is the best bid and ask, ungrouped, so the spread stays true at any grouping.
type BBO struct {
	Coin   string
	Levels [2]Level `json:"bbo"`
}

// L2Change is an update from the website's undocumented "l2" book feed:
// drop the levels at R's indices, then set L's levels by price (size 0 removes).
type L2Change struct {
	C string // coin
	L [2][]L2Level
	R [2][]int
}

type L2Level struct {
	P, S float64 `json:",string"`
}

// Apply returns the book after c, which Hyperliquid sends every half second.
func (b L2Book) Apply(c L2Change) L2Book {
	for side, lv := range b.Levels {
		var out []Level
		for i, l := range lv {
			if !slices.Contains(c.R[side], i) {
				out = append(out, l)
			}
		}
		for _, l := range c.L[side] {
			out = slices.DeleteFunc(out, func(o Level) bool { return o.Px == l.P })
			if l.S > 0 {
				out = append(out, Level{l.P, l.S})
			}
		}
		slices.SortFunc(out, func(x, y Level) int { return cmp.Compare(x.Px, y.Px) * (2*side - 1) }) // bids desc, asks asc
		b.Levels[side] = out[:min(len(out), 20)]
	}
	return b
}

type Trade struct {
	Coin   string
	Side   string  // B buy, A sell
	Px, Sz float64 `json:",string"`
	Time   int64
	Tid    int64 // trade ID
}

type Candle struct {
	Time          int64   `json:"t"`
	End           int64   `json:"T"`
	S, I          string  // coin, interval
	O, H, L, C, V float64 `json:",string"`
}

type Funding struct {
	Coin        string
	FundingRate float64 `json:",string"`
	Time        int64
}

// CoinOf names the market a message is about, so late ones from the
// previous market can be dropped.
func CoinOf(msg any) string {
	switch m := msg.(type) {
	case ActiveAssetCtx:
		return m.Coin
	case L2Book:
		return m.Coin
	case BBO:
		return m.Coin
	case L2Change:
		return m.C
	case Candle:
		return m.S
	case []Candle:
		if len(m) > 0 {
			return m[0].S
		}
	case []Trade:
		if len(m) > 0 {
			return m[0].Coin
		}
	case []Funding:
		if len(m) > 0 {
			return m[0].Coin
		}
	}
	return ""
}

type Interval struct {
	Name string
	Dur  time.Duration
}

var Intervals = []Interval{
	{"1m", time.Minute}, {"3m", 3 * time.Minute}, {"5m", 5 * time.Minute}, {"15m", 15 * time.Minute},
	{"30m", 30 * time.Minute}, {"1h", time.Hour}, {"2h", 2 * time.Hour}, {"4h", 4 * time.Hour},
	{"8h", 8 * time.Hour}, {"12h", 12 * time.Hour}, {"1d", 24 * time.Hour}, {"3d", 72 * time.Hour},
	{"1w", 168 * time.Hour}, {"1M", 720 * time.Hour},
}

// FundingIntervals sum hourly funding per period, like Hyperliquid's funding chart.
var FundingIntervals = []Interval{{"1h", time.Hour}, {"8h", 8 * time.Hour}, {"1d", 24 * time.Hour}}

// Grouping merges book levels to S significant figures, in steps of M (2 or 5) at 5.
type Grouping struct{ S, M int }

var Groupings = []Grouping{{5, 1}, {5, 2}, {5, 5}, {4, 1}, {3, 1}, {2, 1}}

func post(body req, out any) error {
	b, _ := json.Marshal(body)
	resp, err := client.Post("https://api.hyperliquid.xyz/info", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hyperliquid: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Markets lists every live perp, busiest first.
func Markets() ([]Asset, error) {
	var meta struct{ Universe []Asset }
	var ctxs []AssetCtx
	if err := post(req{"type": "metaAndAssetCtxs"}, &[]any{&meta, &ctxs}); err != nil {
		return nil, err
	}
	var ms []Asset
	for i, m := range meta.Universe {
		if !m.IsDelisted && i < len(ctxs) {
			m.AssetCtx = ctxs[i]
			ms = append(ms, m)
		}
	}
	slices.SortFunc(ms, func(a, b Asset) int { return cmp.Compare(b.DayNtlVlm, a.DayNtlVlm) })
	return ms, nil
}

func Market(coin string) (Asset, error) {
	ms, err := Markets()
	if i := slices.IndexFunc(ms, func(m Asset) bool { return strings.EqualFold(m.Name, coin) }); i >= 0 {
		return ms[i], nil
	}
	return Asset{}, cmp.Or(err, fmt.Errorf("%s not found", coin))
}

func Candles(coin string, iv Interval) (c []Candle, err error) {
	end := time.Now()
	err = post(req{"type": "candleSnapshot", "req": req{
		"coin": coin, "interval": iv.Name,
		"startTime": end.Add(-300 * iv.Dur).UnixMilli(), "endTime": end.UnixMilli(),
	}}, &c)
	return
}

// FundingHistory returns coin's hourly funding since a time, 500 hours (the most one call allows) at a time.
func FundingHistory(coin string, since time.Time) ([]Funding, error) {
	var all []Funding
	for start := since.UnixMilli(); ; {
		var page []Funding
		if err := post(req{"type": "fundingHistory", "coin": coin, "startTime": start}, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < 500 {
			return all, nil
		}
		start = page[len(page)-1].Time + 1
	}
}
