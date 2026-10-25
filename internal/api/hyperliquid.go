package api

import (
	"bytes"
	"cmp"
	"compress/flate"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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

type ActiveAssetCtx struct{ Ctx AssetCtx }

type Level struct {
	Px, Sz float64 `json:",string"`
}

type L2Book struct {
	Levels [2][]Level // bids, asks
}

// L2Change is an update from the website's undocumented "l2" book feed:
// drop the levels at R's indices, then set L's levels by price (size 0 removes).
type L2Change struct {
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
	Side   string  // B buy, A sell
	Px, Sz float64 `json:",string"`
	Time   int64
}

type Candle struct {
	Time          int64   `json:"t"`
	End           int64   `json:"T"`
	I             string  // interval
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

func Candles(coin string, iv Interval) (c []Candle, err error) {
	end := time.Now()
	err = post(req{"type": "candleSnapshot", "req": req{
		"coin": coin, "interval": iv.Name,
		"startTime": end.Add(-300 * iv.Dur).UnixMilli(), "endTime": end.UnixMilli(),
	}}, &c)
	return
}

var channels = map[string]func([]byte) (any, error){
	"activeAssetCtx": decode[ActiveAssetCtx],
	"l2":             l2,
	"trades":         decode[[]Trade],
	"candle":         decode[Candle],
}

// l2 decodes the book feed: an L2Book snapshot, then L2Changes that arrive
// plain or deflate-compressed.
func l2(b []byte) (any, error) {
	var m struct {
		S *L2Book
		U L2Change
		C []byte
	}
	err := json.Unmarshal(b, &m)
	switch {
	case err != nil:
		return nil, err
	case m.S != nil:
		return *m.S, nil
	case m.C != nil:
		err = json.NewDecoder(flate.NewReader(bytes.NewReader(m.C))).Decode(&m.U)
	}
	return m.U, err
}

// Watch streams coin's context, book, trades and candles for every interval
// to send, returning when the connection drops or goes quiet for a minute.
func Watch(coin string, send func(any)) error {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.hyperliquid.xyz/ws", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	sub := func(method string, s req) error { return c.WriteJSON(req{"method": method, "subscription": s}) }
	book := req{"type": "l2", "c": coin}
	subs := []req{{"type": "activeAssetCtx", "coin": coin}, {"type": "trades", "coin": coin}, book}
	for _, iv := range Intervals {
		subs = append(subs, req{"type": "candle", "coin": coin, "interval": iv.Name})
	}
	for _, s := range subs {
		if err := sub("subscribe", s); err != nil {
			return err
		}
	}
	for synced := time.Now(); ; {
		var m struct {
			Channel string
			Data    json.RawMessage
		}
		c.SetReadDeadline(time.Now().Add(time.Minute))
		if err := c.ReadJSON(&m); err != nil {
			return err
		}
		if f := channels[m.Channel]; f != nil {
			v, err := f(m.Data)
			if err != nil {
				return err
			}
			send(v)
		}
		// l2 only sends changes, so resubscribe each minute for a fresh snapshot
		if time.Since(synced) > time.Minute {
			synced = time.Now()
			if err := errors.Join(sub("unsubscribe", book), sub("subscribe", book)); err != nil {
				return err
			}
		}
	}
}

func decode[T any](b []byte) (any, error) {
	var v T
	err := json.Unmarshal(b, &v)
	return v, err
}
