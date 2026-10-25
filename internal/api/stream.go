package api

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Stream keeps one websocket open, changing only the subscriptions Watch changes and resubscribing after a reconnect.
type Stream struct {
	mu   sync.Mutex
	conn *websocket.Conn
	subs []req
}

// Watch streams every market's context and, given a coin, its trades, grouped book and candles at iv.
func (s *Stream) Watch(coin string, iv Interval, g Grouping) {
	subs := []req{{"type": "pac"}, {"type": "sac"}, {"type": "fastAssetCtxs"}}
	if coin != "" {
		book := req{"type": "l2", "c": coin, "s": g.S}
		if g.M > 1 {
			book["m"] = g.M
		}
		subs = append(subs, req{"type": "activeAssetCtx", "coin": coin}, req{"type": "trades", "coin": coin}, req{"type": "bbo", "coin": coin}, book, req{"type": "candle", "coin": coin, "interval": iv.Name})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.send("unsubscribe", without(s.subs, subs))
	s.send("subscribe", without(subs, s.subs))
	s.subs = subs
}

// Run sends every update to send, reconnecting 3s after a drop
// (Hyperliquid allows 30 new connections a minute).
func (s *Stream) Run(send func(any)) {
	for {
		send(s.run(send))
		time.Sleep(3 * time.Second)
	}
}

func (s *Stream) run(send func(any)) error {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.hyperliquid.xyz/ws", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	s.mu.Lock()
	s.conn = c
	s.send("subscribe", s.subs)
	s.mu.Unlock()
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
			s.mu.Lock()
			book := slices.DeleteFunc(slices.Clone(s.subs), func(r req) bool { return r["type"] != "l2" })
			s.send("unsubscribe", book)
			s.send("subscribe", book)
			s.mu.Unlock()
		}
	}
}

// send writes subscription changes; a failed write surfaces as a read error in run.
func (s *Stream) send(method string, subs []req) {
	for _, r := range subs {
		if s.conn != nil {
			s.conn.WriteJSON(req{"method": method, "subscription": r})
		}
	}
}

func without(a, b []req) []req {
	return slices.DeleteFunc(slices.Clone(a), func(r req) bool {
		return slices.ContainsFunc(b, func(o req) bool { return fmt.Sprint(o) == fmt.Sprint(r) })
	})
}

var channels = map[string]func([]byte) (any, error){
	"activeAssetCtx":     decode[ActiveAssetCtx],
	"activeSpotAssetCtx": decode[ActiveAssetCtx],
	"l2":                 l2,
	"bbo":                decode[BBO],
	"trades":             decode[[]Trade],
	"candle":             decode[Candle],
	"pac":                dexs,
	"sac":                coins,
	"fastAssetCtxs":      coins,
}

// l2 decodes the book feed: an L2Book snapshot, then L2Changes, plain or deflate-compressed.
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

// dexs decodes the pac feed, [[dex, [ctx...]]...]; coins the sac and fastAssetCtxs feeds, {coin: ctx}.
func dexs(b []byte) (any, error) {
	var pairs []json.RawMessage
	err := inflate(b, &pairs)
	c := Ctxs{Dexs: map[string][]json.RawMessage{}}
	for _, p := range pairs {
		var dex string
		var ctxs []json.RawMessage
		json.Unmarshal(p, &[]any{&dex, &ctxs})
		c.Dexs[dex] = ctxs
	}
	return c, err
}

func coins(b []byte) (any, error) {
	var c Ctxs
	err := inflate(b, &c.Coins)
	return c, err
}

// inflate decodes the website feeds' framing: a base64 string of raw-deflated JSON.
func inflate(b []byte, v any) error {
	var z []byte
	if err := json.Unmarshal(b, &z); err != nil {
		return err
	}
	return json.NewDecoder(flate.NewReader(bytes.NewReader(z))).Decode(v)
}

func decode[T any](b []byte) (any, error) {
	var v T
	err := json.Unmarshal(b, &v)
	return v, err
}
