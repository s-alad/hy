package api

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

type req map[string]any

type AssetCtx struct {
	MarkPx, OraclePx, PrevDayPx, Funding, OpenInterest, DayNtlVlm, CirculatingSupply, TotalSupply float64 `json:",string"`
}

// Asset is a market: a perp on Hyperliquid or on a builder's dex (HIP-3), or a spot pair.
type Asset struct {
	Name        string  // the API's id: BTC, xyz:CL, @107
	Base, Quote string  // the website's names: WTIOIL and USDC for xyz:CL
	Dex         string  // builder dex, empty on Hyperliquid's own perps
	Category    string  // HIP-3 only: stocks, commodities, indices...
	Contract    string  // spot only: the base token's id
	Gecko       float64 // spot only: CoinGecko's market cap, where the website uses it
	Spot        bool
	SzDecimals  int
	MaxLeverage int
	IsDelisted  bool
	AssetCtx
}

var (
	// geckos are the tokens whose market cap the website takes from CoinGecko.
	geckos = map[string]string{"BTC": "bitcoin", "ETH": "ethereum", "SOL": "solana", "FARTCOIN": "fartcoin", "PUMP": "pump-fun", "BONK": "bonk", "XPL": "plasma", "2Z": "doublezero", "MON": "monad", "ENA": "ethena", "ZEC": "zcash", "AVAX": "avalanche-2", "ANSEM": "the-black-bull", "SYN": "synapse-2"}
	// excluded are the tokens the website shows no market cap for (placeholder supplies).
	excluded = strings.Fields("AXL AAVE0 AVAX0 EX FEUSD LINK0 LTHREE MNT NEAR PEG PENGU QQQM REI SLAY SOLV SPX SPCXD THBILL USDE USDH USDHL USDT USDXL USH USR XAUT SEDA BZEC KHYPE MON MOVE STABLE SOON BNB0 AZTEC QONE SEI BASED DIME HMT NIL LMTS DRV MUX NVDAX SKHYX QQQX SPYX")
)

// Cap is a spot token's market cap by the website's rule: CoinGecko's figure if it
// has one, else price × circulating supply (with its two corrections) unless excluded or over $100B.
func (a Asset) Cap() float64 {
	if a.Gecko > 0 {
		return a.Gecko
	}
	supply := a.CirculatingSupply
	switch a.Base {
	case "KNTQ":
		supply -= 730e6
	case "VIRTUAL":
		supply = 656301970
	}
	if v := a.MarkPx * supply; a.Spot && v <= 1e11 && !slices.Contains(excluded, a.Base) {
		return v
	}
	return 0
}

// gecko fetches the market caps the website takes from CoinGecko; empty when that fails.
func gecko() map[string]float64 {
	var ids []string
	for _, id := range geckos {
		ids = append(ids, id)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get("https://api.coingecko.com/api/v3/simple/price?ids=" + strings.Join(ids, ",") + "&vs_currencies=usd&include_market_cap=true")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var got map[string]struct {
		Cap float64 `json:"usd_market_cap"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	caps := map[string]float64{}
	for name, id := range geckos {
		caps[name] = got[id].Cap
	}
	return caps
}

// Pair names the market as the website does: BTC-USDC, HYPE/USDC.
func (a Asset) Pair() string {
	if a.Spot {
		return a.Base + "/" + a.Quote
	}
	return a.Base + "-" + a.Quote
}

func (a Asset) Link() string {
	if a.Spot {
		return "https://app.hyperliquid.xyz/trade/" + a.Base + "/" + a.Quote
	}
	return "https://app.hyperliquid.xyz/trade/" + a.Name
}

// Kind labels the market in lists: perp, spot, or its dex.
func (a Asset) Kind() string {
	switch {
	case a.Spot:
		return "spot"
	case a.Dex != "":
		return a.Dex
	}
	return "perp"
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
	Tid    int64
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

// Coin names the market a message is about, so late ones from the previous market can be dropped.
func Coin(msg any) string {
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

// aliases are the website's display names for wrapped spot tokens, by token index (UBTC shows as BTC).
var aliases = map[int]string{197: "BTC", 221: "ETH", 254: "SOL", 269: "FARTCOIN", 268: "USDT", 292: "PENGU", 297: "XAUT", 299: "PUMP", 26: "PUMP-26", 19: "ANSEM-19", 293: "PEAR", 71: "PEAR-71", 844: "ANSEM", 319: "SPX", 320: "BONK", 246: "MNT", 343: "XPL", 361: "2Z", 338: "ENA", 164: "MONPRO", 383: "MON", 395: "MOVE", 419: "ZEC", 437: "SEI", 443: "VIRTUAL", 451: "AVAX", 513: "NEAR", 842: "NIL"}

type token struct {
	Name, TokenID     string
	Index, SzDecimals int
}

func (t token) display() string { return cmp.Or(aliases[t.Index], t.Name) }

type meta struct {
	Universe        []Asset
	CollateralToken int // the dex's quote, as a spot token index
}

func dex(name string) string {
	d, _, ok := strings.Cut(name, ":")
	if !ok {
		return ""
	}
	return d
}

// Markets lists every live market on every dex, busiest first. It costs 160 of
// Hyperliquid's 1200 request weight a minute, so callers keep the list a while.
func Markets() ([]Asset, error) {
	var metas []meta // Hyperliquid's own perps, then each builder's dex
	if err := post(req{"type": "allPerpMetas"}, &metas); err != nil {
		return nil, err
	}
	metas = slices.DeleteFunc(metas, func(m meta) bool {
		return !slices.ContainsFunc(m.Universe, func(a Asset) bool { return !a.IsDelisted })
	})
	ctxs := make([][]AssetCtx, len(metas))
	var spot struct {
		Universe []struct {
			Name   string
			Tokens [2]int
		}
		Tokens []token
	}
	var sctxs []struct {
		Coin string
		AssetCtx
	}
	var notes [][2]json.RawMessage // [name, {displayName, category}] for HIP-3 assets

	errs := make([]error, len(metas)+2)
	var wg sync.WaitGroup
	fetch := func(i int, body req, out any) {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = post(body, out) }()
	}
	for i, m := range metas {
		fetch(i, req{"type": "metaAndAssetCtxs", "dex": dex(m.Universe[0].Name)}, &[]any{new(struct{}), &ctxs[i]})
	}
	fetch(len(metas), req{"type": "spotMetaAndAssetCtxs"}, &[]any{&spot, &sctxs})
	fetch(len(metas)+1, req{"type": "perpConciseAnnotations"}, &notes)
	var caps map[string]float64
	wg.Add(1)
	go func() { defer wg.Done(); caps = gecko() }()
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	tokens := map[int]token{}
	for _, t := range spot.Tokens {
		tokens[t.Index] = t
	}
	info := map[string]struct{ DisplayName, Category string }{}
	for _, n := range notes {
		var name string
		var v struct{ DisplayName, Category string }
		if json.Unmarshal(n[0], &name) == nil && json.Unmarshal(n[1], &v) == nil {
			info[name] = v
		}
	}
	var ms []Asset
	for i, m := range metas {
		for j, a := range m.Universe {
			if a.IsDelisted || j >= len(ctxs[i]) {
				continue
			}
			a.Dex, a.AssetCtx, a.Category = dex(a.Name), ctxs[i][j], info[a.Name].Category
			a.Base = cmp.Or(info[a.Name].DisplayName, strings.TrimPrefix(a.Name, a.Dex+":"))
			a.Quote = tokens[m.CollateralToken].display()
			ms = append(ms, a)
		}
	}
	ctx := map[string]AssetCtx{}
	for _, c := range sctxs {
		ctx[c.Coin] = c.AssetCtx
	}
	for _, p := range spot.Universe {
		b, q := tokens[p.Tokens[0]], tokens[p.Tokens[1]]
		ms = append(ms, Asset{Name: p.Name, Base: b.display(), Quote: q.display(), Contract: b.TokenID, Gecko: caps[b.display()], Spot: true, SzDecimals: b.SzDecimals, AssetCtx: ctx[p.Name]})
	}
	slices.SortFunc(ms, func(a, b Asset) int { return cmp.Compare(b.DayNtlVlm, a.DayNtlVlm) })
	return ms, nil
}

func Market(q string) (Asset, error) {
	ms, err := Markets()
	if err != nil {
		return Asset{}, err
	}
	return Find(ms, q)
}

// Find looks up a market by any name people use: the API's (xyz:CL, @107), the website's
// pair (BTC-USDC, HYPE/USDC) or the base alone (btc, wtioil, cl), perps before spot.
func Find(ms []Asset, q string) (Asset, error) {
	for _, spot := range []bool{false, true} {
		for _, m := range ms {
			if m.Spot == spot && slices.ContainsFunc([]string{m.Name, m.Pair(), m.Base, strings.TrimPrefix(m.Name, m.Dex+":")}, func(n string) bool { return strings.EqualFold(n, q) }) {
				return m, nil
			}
		}
	}
	return Asset{}, fmt.Errorf("%s not found", q)
}

func Candles(coin string, iv Interval) (c []Candle, err error) {
	end := time.Now()
	err = post(req{"type": "candleSnapshot", "req": req{
		"coin": coin, "interval": iv.Name,
		"startTime": end.Add(-300 * iv.Dur).UnixMilli(), "endTime": end.UnixMilli(),
	}}, &c)
	return
}

// FundingHistory returns coin's hourly funding since a time, 500 hours (one call's limit) at a time.
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
