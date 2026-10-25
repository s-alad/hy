package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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

// Merge overlays b on the book, keeping its levels deeper than b reaches.
// The fast feed sends 5 levels every half second; all 20 come every ~5s.
func (old L2Book) Merge(b L2Book) L2Book {
	for side, lv := range b.Levels {
		if len(lv) == 0 {
			continue
		}
		edge := lv[len(lv)-1].Px
		for _, l := range old.Levels[side] {
			if side == 0 && l.Px < edge || side == 1 && l.Px > edge {
				lv = append(lv, l)
			}
		}
		b.Levels[side] = lv[:min(len(lv), 20)]
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
	"l2Book":         decode[L2Book],
	"trades":         decode[[]Trade],
	"candle":         decode[Candle],
}

// Watch streams coin's context, book, trades and candles for every interval
// to send, returning when the connection drops or goes quiet for a minute.
func Watch(coin string, send func(any)) error {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.hyperliquid.xyz/ws", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	subs := []req{
		{"type": "activeAssetCtx", "coin": coin}, {"type": "trades", "coin": coin},
		{"type": "l2Book", "coin": coin}, {"type": "l2Book", "coin": coin, "fast": true},
	}
	for _, iv := range Intervals {
		subs = append(subs, req{"type": "candle", "coin": coin, "interval": iv.Name})
	}
	for _, s := range subs {
		if err := c.WriteJSON(req{"method": "subscribe", "subscription": s}); err != nil {
			return err
		}
	}
	for {
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
	}
}

func decode[T any](b []byte) (any, error) {
	var v T
	err := json.Unmarshal(b, &v)
	return v, err
}
