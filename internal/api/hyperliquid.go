package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 10 * time.Second}

type req map[string]any

type AssetCtx struct {
	MarkPx, OraclePx, PrevDayPx, Funding, OpenInterest, DayNtlVlm float64 `json:",string"`
}

type Asset struct {
	Name       string
	SzDecimals int
	AssetCtx
}

type Level struct {
	Px, Sz float64 `json:",string"`
}

type L2Book struct {
	Levels [2][]Level // bids, asks
}

type Trade struct {
	Side   string  // B buy, A sell
	Px, Sz float64 `json:",string"`
	Time   int64
}

type Candle struct {
	Time          int64   `json:"t"`
	End           int64   `json:"T"`
	O, H, L, C, V float64 `json:",string"`
}

type Interval struct {
	Name string
	Dur  time.Duration
}

var Intervals = []Interval{
	{"1m", time.Minute}, {"5m", 5 * time.Minute}, {"15m", 15 * time.Minute},
	{"1h", time.Hour}, {"4h", 4 * time.Hour}, {"1d", 24 * time.Hour},
}

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

func Market(coin string) (Asset, error) {
	var meta struct{ Universe []Asset }
	var ctxs []AssetCtx
	if err := post(req{"type": "metaAndAssetCtxs"}, &[]any{&meta, &ctxs}); err != nil {
		return Asset{}, err
	}
	for i, m := range meta.Universe {
		if strings.EqualFold(m.Name, coin) && i < len(ctxs) {
			m.AssetCtx = ctxs[i]
			return m, nil
		}
	}
	return Asset{}, fmt.Errorf("%s not found", coin)
}

func Token(name string) (string, error) {
	var meta struct {
		Tokens []struct{ Name, TokenID string }
	}
	if err := post(req{"type": "spotMeta"}, &meta); err != nil {
		return "", err
	}
	for _, t := range meta.Tokens {
		if t.Name == name {
			return t.TokenID, nil
		}
	}
	return "", nil
}

func Book(coin string) (b L2Book, err error) {
	err = post(req{"type": "l2Book", "coin": coin}, &b)
	return
}

func Trades(coin string) (t []Trade, err error) {
	err = post(req{"type": "recentTrades", "coin": coin}, &t)
	return
}

func Candles(coin string, iv Interval) (c []Candle, err error) {
	end := time.Now()
	err = post(req{"type": "candleSnapshot", "req": req{
		"coin": coin, "interval": iv.Name,
		"startTime": end.Add(-300 * iv.Dur).UnixMilli(), "endTime": end.UnixMilli(),
	}}, &c)
	return
}
